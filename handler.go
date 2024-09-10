package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

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
	// Obtener la colección de estudios
	studiesCollection := db.Collection("estudios")

	// Obtener parámetros de consulta
	queryParams := r.URL.Query()
	tipoEstudio := queryParams.Get("tipoEstudio")
	region := queryParams.Get("region")
	edadMin := queryParams.Get("edadMin")
	edadMax := queryParams.Get("edadMax")
	sexo := queryParams.Get("sexo")

	// Crear el filtro de búsqueda para estudios
	filter := bson.M{
		"imagenes": bson.M{
			"$elemMatch": bson.M{
				"anonimizada": true,
			},
		},
		"status": "Aceptado",
	}

	// Agregar filtros opcionales a la consulta de estudios
	if tipoEstudio != "" {
		filter["estudio"] = tipoEstudio
	}
	if region != "" {
		filter["region"] = region
	}
	if edadMin != "" {
		edadMinInt, err := strconv.Atoi(edadMin)
		if err != nil {
			http.Error(w, "Edad mínima inválida", http.StatusBadRequest)
			return
		}
		filter["edad"] = bson.M{"$gte": edadMinInt}
	}
	if edadMax != "" {
		edadMaxInt, err := strconv.Atoi(edadMax)
		if err != nil {
			http.Error(w, "Edad máxima inválida", http.StatusBadRequest)
			return
		}
		filter["edad"] = bson.M{"$lte": edadMaxInt}
	}
	if sexo != "" {
		filter["sexo"] = sexo
	}

	// Buscar los estudios que cumplen con el filtro
	cursor, err := studiesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar estudios en la base de datos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	// Recolectar IDs de imágenes que cumplen con los criterios
	var imageIDs []primitive.ObjectID
	for cursor.Next(context.Background()) {
		var study EstudioDocument
		if err := cursor.Decode(&study); err != nil {
			http.Error(w, "Error al decodificar estudio: "+err.Error(), http.StatusInternalServerError)
			return
		}

		for _, img := range study.Imagenes {
			if img.Anonimizada {
				imageID, err := primitive.ObjectIDFromHex(img.Imagen)
				if err != nil {
					http.Error(w, "Error al convertir ID de imagen: "+err.Error(), http.StatusInternalServerError)
					return
				}
				imageIDs = append(imageIDs, imageID)
			}
		}
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

	// Obtener la colección de archivos GridFS
	imagesCollection := db.Collection("imagenes.files")

	// Filtrar archivos con IDs en imageIDs y que terminen en .jpg
	filter = bson.M{
		"_id":      bson.M{"$in": imageIDs},
		"filename": bson.M{"$regex": `\.jpg$`},
	}

	// Buscar los archivos en la colección usando el filtro
	cursor, err = imagesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar archivos en la base de datos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var images []string
	for cursor.Next(context.Background()) {
		var fileInfo FileDocument
		if err := cursor.Decode(&fileInfo); err != nil {
			http.Error(w, "Error al decodificar archivo: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Obtener el nombre del archivo y construir la URL
		filename := fileInfo.Filename
		if filename != "" {
			imageURL := ip + "/image/" + filename
			images = append(images, imageURL)
		}
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Error al iterar sobre los archivos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Devolver la lista de URLs de las miniaturas
	w.Header().Set("Content-Type", "application/json")
	if len(images) == 0 {
		log.Println("No se encontraron imágenes que coincidan con los filtros.")
	}
	json.NewEncoder(w).Encode(images)
}
func handleImportar(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket, database *mongo.Database) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Limitar el tamaño del archivo a 50 MB
	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		log.Println("Error parsing form data:", err)
		return
	}

	formData := r.MultipartForm
	log.Println("Form Data:", formData)

	// Verificar campos obligatorios
	estudioID, err := getValueOrError(formData.Value, "estudio_ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	estudio, err := getValueOrError(formData.Value, "estudio")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	region, err := getValueOrError(formData.Value, "region")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sexo, err := getValueOrError(formData.Value, "sexo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	edadStr, err := getValueOrError(formData.Value, "edad")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fechaNacimiento, err := getValueOrError(formData.Value, "fecha_nacimiento")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	FechaEstudio, err := getValueOrError(formData.Value, "fecha_estudio")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	proyeccion, err := getValueOrError(formData.Value, "proyeccion")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hallazgos := formData.Value["hallazgos"]
	if len(hallazgos) == 0 {
		hallazgos = []string{"N/A"} // Valor por defecto si hallazgos no está presente
	}

	donador, err := getValueOrError(formData.Value, "donador")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	numeroOperacion, err := getValueOrError(formData.Value, "estudio_ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Conversión de edad
	edad, err := strconv.Atoi(edadStr)
	if err != nil {
		http.Error(w, "Invalid age format", http.StatusBadRequest)
		log.Println("Invalid age format:", err)
		return
	}

	// Conversión de fecha
	fechaNacimientoParsed, err := time.Parse("2006-01-02", fechaNacimiento)
	if err != nil {
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		log.Println("Invalid date format:", err)
		return
	}

	// Conversión de fecha
	fechaEstudioParsed, err := time.Parse("2006-01-02", FechaEstudio)
	if err != nil {
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		log.Println("Invalid date format:", err)
		return
	}

	// Generar el hash a partir del nombre del donador y el número de operación
	hash := generateHash(donador, numeroOperacion)
	log.Println("Generated Hash:", hash)

	// Procesamiento de archivos originales
	originalFiles := formData.File["archivosOriginales"]
	anonymizedFiles := formData.File["archivosAnonimizados"]

	if originalFiles == nil || anonymizedFiles == nil {
		http.Error(w, "No images field found", http.StatusBadRequest)
		log.Println("No 'archivosOriginales' or 'archivosAnonimizados' field found in form data")
		return
	}

	log.Println("Original Files received:", len(originalFiles))
	log.Println("Anonymized Files received:", len(anonymizedFiles))

	if len(originalFiles) == 0 || len(anonymizedFiles) == 0 {
		http.Error(w, "No images uploaded", http.StatusBadRequest)
		log.Println("No images received")
		return
	}

	var imagenes []Imagen

	// Subir archivos originales
	for _, fileHeader := range originalFiles {
		log.Printf("Processing original file: %s", fileHeader.Filename)

		fileID, err := uploadFileToGridFS(fileHeader, bucket)
		if err != nil {
			http.Error(w, "Failed to upload original file", http.StatusInternalServerError)
			return
		}

		imagenes = append(imagenes, Imagen{
			Imagen:      fileID,
			Anonimizada: false, // Archivo original
		})
	}

	// Subir archivos anonimizados
	for _, fileHeader := range anonymizedFiles {
		log.Printf("Processing anonymized file: %s", fileHeader.Filename)

		fileID, err := uploadFileToGridFS(fileHeader, bucket)
		if err != nil {
			http.Error(w, "Failed to upload anonymized file", http.StatusInternalServerError)
			return
		}

		imagenes = append(imagenes, Imagen{
			Imagen:      fileID,
			Anonimizada: true, // Archivo anonimizado
		})
	}

	// Crear el documento del estudio
	estudioDoc := EstudioDocument{
		EstudioID:       estudioID,
		Region:          region,
		Hash:            hash, // Asignar el hash generado
		Status:          "No Aceptado",
		Estudio:         estudio,
		Sexo:            sexo,
		Edad:            edad,
		FechaNacimiento: fechaNacimientoParsed,
		FechaEstudio:    fechaEstudioParsed,
		Imagenes:        imagenes,
		Diagnostico: []Diagnostico{
			{
				Proyeccion: proyeccion,
				Hallazgos:  hallazgos[0],
			},
		},
	}

	// Insertar el documento en MongoDB
	collection := database.Collection("estudios")
	_, err = collection.InsertOne(r.Context(), estudioDoc)
	if err != nil {
		http.Error(w, "Failed to insert document", http.StatusInternalServerError)
		log.Println("Error inserting document into MongoDB:", err)
		return
	}

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

// Función para generar un hash SHA-256
func generateHash(donador, numOperacion string) string {
	hashInput := donador + numOperacion
	hash := sha256.New()
	hash.Write([]byte(hashInput))
	return hex.EncodeToString(hash.Sum(nil))
}

// Función para subir archivos a GridFS
func uploadFileToGridFS(fileHeader *multipart.FileHeader, bucket *gridfs.Bucket) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		log.Println("Error opening file:", err)
		return "", err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		log.Println("Error decoding image:", err)
		return "", err
	}

	resizedImg := imaging.Resize(img, 4096, 4096, imaging.Lanczos)

	var resizedImageBuf bytes.Buffer
	if err := jpeg.Encode(&resizedImageBuf, resizedImg, nil); err != nil {
		log.Println("Error encoding resized image:", err)
		return "", err
	}

	uploadStream, err := bucket.OpenUploadStream(fileHeader.Filename)
	if err != nil {
		log.Println("Error uploading image to GridFS:", err)
		return "", err
	}
	defer uploadStream.Close()

	_, err = io.Copy(uploadStream, &resizedImageBuf)
	if err != nil {
		log.Println("Error copying image to GridFS:", err)
		return "", err
	}

	return uploadStream.FileID.(primitive.ObjectID).Hex(), nil
}
