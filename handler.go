package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"os/exec"
<<<<<<< Updated upstream
	"path/filepath"
	"runtime"
=======
	"strconv"
	"time"
>>>>>>> Stashed changes

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

<<<<<<< Updated upstream
// DonacionHandler maneja la solicitud para cargar archivos a una carpeta local.
// DonacionHandler maneja la solicitud para cargar archivos y ejecutar el script de Python.
func DonacionHandler(w http.ResponseWriter, r *http.Request) {
=======
func handleImportar(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket, database *mongo.Database) {
>>>>>>> Stashed changes
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

<<<<<<< Updated upstream
	// Aumentar el límite de tamaño de los archivos a 50 MB
	err := r.ParseMultipartForm(50 << 20) // Limita a 50 MB
=======
	// Parsear el formulario multipart para manejar archivos grandes (hasta 10MB)
	err := r.ParseMultipartForm(10 << 20)
>>>>>>> Stashed changes
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		fmt.Println("Error parsing form data:", err)
		return
	}

<<<<<<< Updated upstream
	// Recuperar los archivos del formulario
	files := r.MultipartForm.File["files"]
	tipoEstudio := r.FormValue("tipoEstudio")
=======
	formData := r.MultipartForm
	fmt.Println("Form Data:", formData) // Verifica lo que se recibe en el formulario
>>>>>>> Stashed changes

	// Verificar si los valores claves están presentes
	if len(formData.Value["estudio_ID"]) == 0 || len(formData.Value["sexo"]) == 0 {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		fmt.Println("Missing estudio_ID or sexo")
		return
	}

	estudioID := formData.Value["estudio_ID"][0]
	estudio := formData.Value["estudio"][0]
	sexo := formData.Value["sexo"][0]
	edadStr := formData.Value["edad"][0]
	fechaNacimiento := formData.Value["fecha_nacimiento"][0]
	proyeccion := formData.Value["proyeccion"][0]
	hallazgos := formData.Value["hallazgos"][0]

	fmt.Println("Estudio ID:", estudioID)
	fmt.Println("Sexo:", sexo)
	fmt.Println("Edad:", edadStr)
	fmt.Println("Fecha Nacimiento:", fechaNacimiento)
	fmt.Println("Proyeccion:", proyeccion)
	fmt.Println("Hallazgos:", hallazgos)

	// Convertir edad de string a int
	edad, err := strconv.Atoi(edadStr)
	if err != nil {
		http.Error(w, "Invalid age format", http.StatusBadRequest)
		fmt.Println("Invalid age format:", err)
		return
	}

	// Convertir fecha de nacimiento de string a time.Time
	fechaNacimientoParsed, err := time.Parse("2006-01-02", fechaNacimiento)
	if err != nil {
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		fmt.Println("Invalid date format:", err)
		return
	}

	// Manejo de los archivos de imágenes
	files := formData.File["imagenes"] // Cambiar a "imagenes" para coincidir con el frontend
	if len(files) == 0 {
		http.Error(w, "No images uploaded", http.StatusBadRequest)
		fmt.Println("No images received")
		return
	}

	fmt.Println("Number of images received:", len(files))

<<<<<<< Updated upstream
	// Guardar los archivos en el directorio
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

	// Ejecutar el script de Python para anonimizar cada archivo en el directorio
	err = processFilesInDirectory(uploadDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al ejecutar el script de Python: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Archivos subidos y anonimización completada exitosamente")
}

func processFilesInDirectory(directory string) error {
	// Lee el directorio especificado
	files, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("error al leer el directorio: %v", err)
	}

	// Determina la ruta del script de Python
	scriptPath := filepath.Join("services", "anonimizacion.py") // Ajusta si el script está en una ubicación diferente

	// Determina la ruta del ejecutable de Python
	var pythonExecutable string
	switch runtime.GOOS {
	case "windows":
		pythonExecutable = "python" // o "python3" dependiendo de tu configuración
	case "linux", "darwin":
		pythonExecutable = "python3"
	default:
		return fmt.Errorf("sistema operativo no soportado: %s", runtime.GOOS)
	}

	for _, file := range files {
		if !file.IsDir() {
			filePath := filepath.Join(directory, file.Name())

			// Ejecuta el script de Python con la ruta del archivo como argumento
			cmd := exec.Command(pythonExecutable, scriptPath, filePath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("error al ejecutar el script de Python para el archivo %s: %v\nOutput: %s", file.Name(), err, output)
			}
		}
	}
	return nil
=======
	var imagenes []Imagen
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Failed to open file", http.StatusInternalServerError)
			fmt.Println("Error opening file:", err)
			return
		}
		defer file.Close()

		// Convertir la imagen JPG original a DICOM
		dicomData, err := convertJPGtoDICOM(file)
		if err != nil {
			http.Error(w, "Failed to convert JPG to DICOM", http.StatusInternalServerError)
			fmt.Println("Error converting JPG to DICOM:", err)
			return
		}

		// Subir el DICOM a GridFS
		dicomUploadStream, err := bucket.OpenUploadStream(fileHeader.Filename + ".dcm")
		if err != nil {
			http.Error(w, "Failed to upload DICOM to GridFS", http.StatusInternalServerError)
			fmt.Println("Error uploading DICOM to GridFS:", err)
			return
		}
		defer dicomUploadStream.Close()

		_, err = io.Copy(dicomUploadStream, dicomData)
		if err != nil {
			http.Error(w, "Failed to write DICOM to GridFS", http.StatusInternalServerError)
			fmt.Println("Error copying DICOM to GridFS:", err)
			return
		}

		// Redimensionar la imagen JPG original a 128x128 píxeles
		img, _, err := image.Decode(file)
		if err != nil {
			http.Error(w, "Failed to decode image", http.StatusInternalServerError)
			fmt.Println("Error decoding image:", err)
			return
		}

		resizedImg := imaging.Resize(img, 128, 128, imaging.Lanczos)

		var resizedImageBuf bytes.Buffer
		if err := jpeg.Encode(&resizedImageBuf, resizedImg, nil); err != nil {
			http.Error(w, "Failed to encode resized image", http.StatusInternalServerError)
			fmt.Println("Error encoding resized image:", err)
			return
		}

		// Subir la imagen redimensionada a GridFS
		uploadStream, err := bucket.OpenUploadStream(fileHeader.Filename)
		if err != nil {
			http.Error(w, "Failed to upload resized image to GridFS", http.StatusInternalServerError)
			fmt.Println("Error uploading resized image to GridFS:", err)
			return
		}
		defer uploadStream.Close()

		_, err = io.Copy(uploadStream, &resizedImageBuf)
		if err != nil {
			http.Error(w, "Failed to write resized image to GridFS", http.StatusInternalServerError)
			fmt.Println("Error copying resized image to GridFS:", err)
			return
		}

		// Convertir el ID a `primitive.ObjectID` y obtener su representación en hexadecimal
		fileID := uploadStream.FileID.(primitive.ObjectID)
		dicomFileID := dicomUploadStream.FileID.(primitive.ObjectID)

		// Agregar la referencia al archivo subido (ID de GridFS)
		fmt.Println("Image uploaded successfully:", fileHeader.Filename, "with ID:", fileID.Hex())
		fmt.Println("DICOM uploaded successfully:", fileHeader.Filename+".dcm", "with ID:", dicomFileID.Hex())

		imagenes = append(imagenes, Imagen{
			Dicom:  dicomFileID.Hex(), // Guardar el ID de GridFS del DICOM en el campo Dicom
			Imagen: fileID.Hex(),      // Guardar el ID de GridFS de la imagen redimensionada en el campo Imagen
		})
	}

	// Crear el documento de estudio
	estudioDoc := EstudioDocument{
		EstudioID:       estudioID,
		Hash:            "",
		Estudio:         estudio,
		Sexo:            sexo,
		Edad:            edad, // Aquí ya es de tipo int
		FechaNacimiento: fechaNacimientoParsed,
		Imagenes:        imagenes,
		Diagnostico: []Diagnostico{
			{
				Proyeccion: proyeccion,
				Hallazgos:  hallazgos,
			},
		},
	}

	// Insertar el documento en la colección "estudios"
	collection := database.Collection("estudios")
	_, err = collection.InsertOne(context.Background(), estudioDoc)
	if err != nil {
		http.Error(w, "Failed to insert document", http.StatusInternalServerError)
		fmt.Println("Error inserting document into MongoDB:", err)
		return
	}

	// Responder con JSON de éxito
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"message": "Data successfully inserted"}
	json.NewEncoder(w).Encode(response)
}

// convertJPGtoDICOM convierte una imagen JPG a DICOM ejecutando un script Python.
func convertJPGtoDICOM(imgData io.Reader) (io.Reader, error) {
	// Crear un archivo temporal para almacenar la imagen JPG
	imgFile, err := os.CreateTemp("", "*.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp JPG file: %w", err)
	}
	defer os.Remove(imgFile.Name())

	// Copiar los datos de la imagen al archivo temporal
	_, err = io.Copy(imgFile, imgData)
	if err != nil {
		return nil, fmt.Errorf("failed to write JPG to temp file: %w", err)
	}
	imgFile.Close()

	// Ejecutar el script Python para convertir JPG a DICOM
	dicomFile := imgFile.Name() + ".dcm"
	cmd := exec.Command("python", "convert_jpg_to_dicom.py", imgFile.Name(), dicomFile)
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("conversion failed: %s", stderr.String())
	}

	// Leer el archivo DICOM generado
	dicomData, err := os.Open(dicomFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open DICOM file: %w", err)
	}

	return dicomData, nil
>>>>>>> Stashed changes
}
