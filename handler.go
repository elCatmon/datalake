package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"golang.org/x/crypto/bcrypt"
)

// RegisterHandler maneja la solicitud de registro de un nuevo usuario.
func RegisterHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")

	log.Println("Iniciando registro de usuario")

	var newUser User
	err := json.NewDecoder(r.Body).Decode(&newUser)
	if err != nil {
		log.Printf("Error al decodificar los datos: %v", err)
		http.Error(w, `{"error": "Error al decodificar los datos"}`, http.StatusBadRequest)
		return
	}

	log.Printf("Usuario recibido: %+v", newUser)

	exists, err := ExisteCorreo(db, newUser.Correo)
	if err != nil {
		log.Printf("Error al verificar el correo: %v", err)
		http.Error(w, `{"error": "Error al verificar el correo"}`, http.StatusInternalServerError)
		return
	}

	if exists {
		response := map[string]string{"error": "El correo ya está en uso"}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newUser.Contrasena), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error al encriptar la contraseña: %v", err)
		http.Error(w, `{"error": "Error al encriptar la contraseña"}`, http.StatusInternalServerError)
		return
	}

	newUser.Contrasena = string(hashedPassword)

	log.Println("Registrando usuario en la base de datos")
	err = RegistrarUsuario(db, newUser)
	if err != nil {
		log.Printf("Error al registrar usuario: %v", err)
		http.Error(w, `{"error": "Error al registrar usuario"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{"message": "Registro exitoso"}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	log.Println("Registro exitoso")
}

// LoginHandler maneja la solicitud de inicio de sesión del usuario.
func LoginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var credentials User
	err := json.NewDecoder(r.Body).Decode(&credentials)
	if err != nil {
		http.Error(w, `{"error": "Error al decodificar los datos"}`, http.StatusBadRequest)
		return
	}

	isValid, id, authErr := ValidarUsuario(db, credentials.Correo, credentials.Contrasena)
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
	downloadStream, err := EncontrarImagen(bucket, filename)
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
	// Obtener la colección de estudios
	studiesCollection := db.Collection("estudios")

	// Obtener parámetros de consulta
	queryParams := r.URL.Query()
	tipoEstudio := queryParams.Get("tipoEstudio")
	region := queryParams.Get("region")
	edadMin := queryParams.Get("edadMin")
	edadMax := queryParams.Get("edadMax")
	sexo := queryParams.Get("sexo")

	//Crear filtro para los estudios
	filter, err := CrearFiltro(w, tipoEstudio, region, edadMin, edadMax, sexo)

	if err != nil {
		http.Error(w, "Error al crear el filtro", http.StatusInternalServerError)
		return
	}

	imageIDs, cursor, error := buscarEstudios(w, studiesCollection, filter)
	if error != nil {
		http.Error(w, "Error al buscar los estudios", http.StatusInternalServerError)
		return
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Error al iterar sobre los estudios: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(imageIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{})
		return
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Error al iterar sobre los archivos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	images, error := BuscarImagenes(w, imageIDs, db)
	if error != nil {

	}

	// Devolver la lista de URLs de las miniaturas
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

func handleImportar(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket, database *mongo.Database) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	// Limitar el tamaño del archivo a 20 MB
	err := r.ParseMultipartForm(20 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}
	//Procesar datos del formulario para subirlos a mongo
	datos, err := ProcesarDonacionFisica(w, r)
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
	}
	//Subir informacion e imagenes a mongo
	SubirDonacionFisica(datos, w, bucket, r, database)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"message": "Data successfully inserted"}
	json.NewEncoder(w).Encode(response)
}

// Función para obtener valores del formulario o devolver un error si el campo no existe
func getValueOrError(formData map[string][]string, key string) (string, error) {
	values, ok := formData[key]
	if !ok || len(values) == 0 {
		return "", errors.New("Missing or empty field: " + key)
	}
	return values[0], nil
}

// Función para subir archivos a GridFS
func uploadFileToGridFS(fileHeader *multipart.FileHeader, bucket *gridfs.Bucket) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", err
	}

	resizedImg := imaging.Resize(img, 4096, 4096, imaging.Lanczos)

	var resizedImageBuf bytes.Buffer
	if err := jpeg.Encode(&resizedImageBuf, resizedImg, nil); err != nil {
		return "", err
	}

	uploadStream, err := bucket.OpenUploadStream(fileHeader.Filename)
	if err != nil {
		return "", err
	}
	defer uploadStream.Close()

	_, err = io.Copy(uploadStream, &resizedImageBuf)
	if err != nil {
		return "", err
	}

	return uploadStream.FileID.(primitive.ObjectID).Hex(), nil
}
