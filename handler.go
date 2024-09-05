package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"

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
