package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"golang.org/x/crypto/bcrypt"
)

// RegisterHandler maneja la solicitud de registro de un nuevo usuario.
func RegisterHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var newUser User
	err := json.NewDecoder(r.Body).Decode(&newUser)
	if err != nil {
		http.Error(w, "Error al decodificar los datos", http.StatusBadRequest)
		return
	}

	// Verificar si el correo ya existe
	exists, err := EmailExists(db, newUser.Correo)
	if err != nil {
		http.Error(w, "Error al verificar el correo", http.StatusInternalServerError)
		return
	}

	if exists {
		response := map[string]string{"error": "El correo ya está en uso"}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Hash de la contraseña
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newUser.Contrasena), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error al encriptar la contraseña", http.StatusInternalServerError)
		return
	}

	// Reemplazar la contraseña en el objeto newUser con la versión encriptada
	newUser.Contrasena = string(hashedPassword)

	// Registrar el usuario en la base de datos.
	err = RegisterUser(db, newUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]string{"message": "Registro exitoso"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// LoginHandler maneja la solicitud de inicio de sesión del usuario.
func LoginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var credentials User
	err := json.NewDecoder(r.Body).Decode(&credentials)
	if err != nil {
		http.Error(w, `{"error": "Error al decodificar los datos"}`, http.StatusBadRequest)
		return
	}

	isValid, id, authErr := AuthenticateUser(db, credentials.Correo, credentials.Contrasena)
	if authErr != nil {
		http.Error(w, `{"error": "Error interno del servidor"}`, http.StatusInternalServerError)
		return
	}

	if !isValid {
		http.Error(w, `{"error": "Correo o contraseña incorrectos"}`, http.StatusUnauthorized)
		return
	}

	response := map[string]string{"message": "Inicio de sesión exitoso", "id": id}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// UploadHandler maneja la solicitud para cargar una imagen a GridFS.
func UploadHandler(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket) {
	// Leer el archivo cargado
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error al leer el archivo", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Crear un nuevo upload stream en GridFS
	uploadStream, err := bucket.OpenUploadStream(r.FormValue("filename"))
	if err != nil {
		http.Error(w, "Error al crear el upload stream", http.StatusInternalServerError)
		return
	}
	defer uploadStream.Close()

	// Copiar el archivo al upload stream
	_, err = io.Copy(uploadStream, file)
	if err != nil {
		http.Error(w, "Error al copiar el archivo", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Archivo cargado exitosamente"))
}

// ImageHandler maneja la solicitud para obtener una imagen por su nombre.
func ImageHandler(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket) {
	filename := mux.Vars(r)["filename"]

	// Buscar el archivo en GridFS
	downloadStream, err := FindImage(bucket, filename)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Archivo no encontrado", http.StatusNotFound)
		} else {
			http.Error(w, "Error al buscar el archivo en la base de datos", http.StatusInternalServerError)
		}
		return
	}
	defer downloadStream.Close()

	// Leer el archivo y enviarlo en la respuesta
	data, err := io.ReadAll(downloadStream)
	if err != nil {
		http.Error(w, "Error al leer el archivo", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ThumbnailHandler maneja la solicitud para obtener las miniaturas de imágenes JPG.
func ThumbnailHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	// Obtener la colección de archivos GridFS
	imagesCollection := db.Collection("imagenes.files")

	// Filtrar solo archivos que terminan en .jpg
	filter := bson.M{"filename": bson.M{"$regex": `\.jpg$`}}

	// Buscar los archivos en la colección usando el filtro
	cursor, err := imagesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar archivos en la base de datos", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var images []string
	for cursor.Next(context.Background()) {
		var fileInfo bson.M
		if err := cursor.Decode(&fileInfo); err != nil {
			http.Error(w, "Error al decodificar archivo", http.StatusInternalServerError)
			return
		}

		// Obtener el nombre del archivo y construir la URL
		filename, ok := fileInfo["filename"].(string)
		if ok {
			imageURL := ip + "/image/" + filename

			images = append(images, imageURL)
		}
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Error al iterar sobre los archivos", http.StatusInternalServerError)
		return
	}

	// Devolver la lista de URLs de las miniaturas
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}
func DonacionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método de solicitud no permitido", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(50 << 20) // Limita a 50 MB
	if err != nil {
		http.Error(w, "No se pudo procesar el formulario", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	tipoEstudio := r.FormValue("tipoEstudio")

	if len(files) == 0 {
		http.Error(w, "No se proporcionaron archivos", http.StatusBadRequest)
		return
	}

	if tipoEstudio == "" {
		http.Error(w, "Tipo de estudio no proporcionado", http.StatusBadRequest)
		return
	}

	for _, file := range files {
		f, err := file.Open()
		if err != nil {
			http.Error(w, "No se pudo abrir el archivo", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		dst, err := os.Create(filepath.Join(uploadDir, file.Filename))
		if err != nil {
			http.Error(w, "No se pudo guardar el archivo", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		_, err = io.Copy(dst, f)
		if err != nil {
			http.Error(w, "Error al guardar el archivo", http.StatusInternalServerError)
			return
		}
	}

	err = processFilesInDirectory(uploadDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al ejecutar el script de Python: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Archivos subidos y anonimización completada exitosamente")
}

func processFilesInDirectory(directory string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("error al leer el directorio: %v", err)
	}

	scriptPath := filepath.Join("services", "anonimizacion.py")
	var pythonExecutable string
	switch runtime.GOOS {
	case "windows":
		pythonExecutable = "python"
	case "linux", "darwin":
		pythonExecutable = "python3"
	}

	for _, file := range files {
		if !file.IsDir() {
			filePath := filepath.Join(directory, file.Name())
			cmd := exec.Command(pythonExecutable, scriptPath, filePath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("error al ejecutar el script de Python para el archivo %s: %v\nOutput: %s", file.Name(), err, output)
			}
		}
	}
	return nil
}

func downloadBlob(url string) (string, error) {
	tempFile, err := os.CreateTemp("", "downloaded-*.jpg")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("descarga fallida con estado %d", resp.StatusCode)
	}

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func convertImagenes(rawImagenes []struct {
	Dicom  string `json:"dicom"`
	Imagen string `json:"imagen"`
}) []Imagen {
	var imagenes []Imagen
	for _, img := range rawImagenes {
		imagenes = append(imagenes, Imagen{
			Dicom:  img.Dicom,
			Imagen: img.Imagen,
		})
	}
	return imagenes
}

func convertDiagnostico(rawDiagnostico []struct {
	Proyeccion string `json:"proyeccion"`
	Hallazgos  string `json:"hallazgos"`
}) []Diagnostico {
	var diagnosticos []Diagnostico
	for _, diag := range rawDiagnostico {
		diagnosticos = append(diagnosticos, Diagnostico{
			Proyeccion: diag.Proyeccion,
			Hallazgos:  diag.Hallazgos,
		})
	}
	return diagnosticos
}

func UploadEstudioHandler(db *mongo.Database, bucket *gridfs.Bucket) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var estudioData struct {
			EstudioID       string `json:"estudio_ID"`
			Hash            string `json:"hash"`
			Estudio         string `json:"estudio"`
			Sexo            string `json:"sexo"`
			Edad            string `json:"edad"`
			FechaNacimiento string `json:"fecha_nacimiento"`
			Imagenes        []struct {
				Dicom  string `json:"dicom"`
				Imagen string `json:"imagen"`
			} `json:"imagenes"`
			Diagnostico []struct {
				Proyeccion string `json:"proyeccion"`
				Hallazgos  string `json:"hallazgos"`
			} `json:"diagnostico"`
		}

		if err := json.NewDecoder(r.Body).Decode(&estudioData); err != nil {
			log.Printf("Error decoding request body: %v", err)
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// Log data decoded
		log.Printf("Decoded Estudio Data: %+v", estudioData)

		edad, err := strconv.Atoi(estudioData.Edad)
		if err != nil {
			log.Printf("Error converting Edad to int: %v", err)
			http.Error(w, "Invalid edad format", http.StatusBadRequest)
			return
		}

		fechaNacimiento, err := time.Parse("2006-01-02", estudioData.FechaNacimiento)
		if err != nil {
			log.Printf("Error parsing FechaNacimiento: %v", err)
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}

		scriptConvPath := filepath.Join("services", "jpg_dcm.py")

		var pythonExecutable string
		switch runtime.GOOS {
		case "windows":
			pythonExecutable = "python"
		case "linux", "darwin":
			pythonExecutable = "python3"
		}

		for i, img := range estudioData.Imagenes {
			i = i
			jpgFilePath, err := downloadBlob(img.Imagen)
			if err != nil {
				log.Printf("Error downloading image: %v", err)
				http.Error(w, "Error downloading image", http.StatusInternalServerError)
				return
			}
			defer os.Remove(jpgFilePath)

			cmd := exec.Command(pythonExecutable, scriptConvPath, jpgFilePath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("Error executing Python script: %v\nOutput: %s", err, output)
				http.Error(w, "Error processing image", http.StatusInternalServerError)
				return
			}
		}

		collection := db.Collection("estudios")
		_, err = collection.InsertOne(context.TODO(), EstudioDocument{
			EstudioID:       estudioData.EstudioID,
			Hash:            estudioData.Hash,
			Estudio:         estudioData.Estudio,
			Sexo:            estudioData.Sexo,
			Edad:            edad,
			FechaNacimiento: fechaNacimiento,
			Imagenes:        convertImagenes(estudioData.Imagenes),
			Diagnostico:     convertDiagnostico(estudioData.Diagnostico),
		})
		if err != nil {
			log.Printf("Error inserting document into MongoDB: %v", err)
			http.Error(w, "Error saving to database", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Estudio cargado exitosamente")
	}
}
