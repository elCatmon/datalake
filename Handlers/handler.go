package Handlers

import (
	"database/sql"
	"encoding/json"

	"io"
	"log"
	"net/http"
	"strconv"

	"webservice/models"
	"webservice/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"golang.org/x/crypto/bcrypt"
)

// RegisterHandler maneja la solicitud de registro de un nuevo usuario.
func RegisterHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	w.Header().Set("Content-Type", "application/json")

	log.Println("Iniciando registro de usuario")

	var newUser models.User
	err := json.NewDecoder(r.Body).Decode(&newUser)
	if err != nil {
		log.Printf("Error al decodificar los datos: %v", err)
		http.Error(w, `{"error": "Error al decodificar los datos"}`, http.StatusBadRequest)
		return
	}

	log.Printf("Usuario recibido: %+v", newUser)

	exists, err := services.ExisteCorreo(db, newUser.Correo)
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
	err = services.RegistrarUsuario(db, newUser)
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
	var credentials models.User
	err := json.NewDecoder(r.Body).Decode(&credentials)
	if err != nil {
		log.Printf("Error al decodificar los datos: %v", err)
		http.Error(w, `{"error": "Error al decodificar los datos"}`, http.StatusBadRequest)
		return
	}

	isValid, id, authErr := services.ValidarUsuario(db, credentials.Correo, credentials.Contrasena)
	if authErr != nil {
		log.Printf("Error al validar usuario: %v", authErr)
		http.Error(w, `{"error": "Error interno del servidor"}`, http.StatusInternalServerError)
		return
	}

	if !isValid {
		log.Printf("Intento de inicio de sesión fallido: correo o contraseña incorrectos para el correo %s", credentials.Correo)
		http.Error(w, `{"error": "Correo o contraseña incorrectos"}`, http.StatusUnauthorized)
		return
	}

	response := map[string]string{"message": "Inicio de sesión exitoso", "id": id}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error al codificar la respuesta JSON: %v", err)
	}
	log.Printf("Inicio de sesión exitoso para el correo %s, ID de usuario: %s", credentials.Correo, id)
}

func UploadHandler(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket, database *mongo.Database) {
	// Ejecutar la función de procesamiento
	err := services.SubirDonacionDigital(w, bucket, r, database)

	// Si ocurrió un error en SubirDonacionDigital, no es necesario llamar a WriteHeader ni Write de nuevo
	if err != nil {
		log.Printf("Error durante la carga: %v", err)
		return // La respuesta de error ya ha sido enviada dentro de SubirDonacionDigital
	}

	// Si todo salió bien, registrar la inserción exitosa
	log.Println("Documento del estudio insertado exitosamente")

	// Enviar la respuesta exitosa
	w.WriteHeader(http.StatusOK) // Solo se llama aquí si todo fue exitoso
	w.Write([]byte("Archivo cargado exitosamente"))
}

// ImageHandler maneja la solicitud para obtener una imagen por su nombre.
func ImageHandler(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket) {
	filename := mux.Vars(r)["filename"]

	// Buscar el archivo en GridFS
	downloadStream, err := services.EncontrarImagen(bucket, filename)
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

	// Parámetros de paginación
	pageStr := queryParams.Get("page")
	limitStr := queryParams.Get("limit")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1 // Página por defecto si el parámetro es inválido
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 18 // Límite por defecto si el parámetro es inválido
	}

	// Crear filtro para los estudios
	filter, err := services.CrearFiltro(w, tipoEstudio, region, edadMin, edadMax, sexo)
	if err != nil {
		http.Error(w, "Error al crear el filtro", http.StatusInternalServerError)
		return
	}

	imageIDs, cursor, err := services.BuscarEstudios(w, studiesCollection, filter)
	if err != nil {
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

	// Aplicar paginación
	start := (page - 1) * limit
	end := start + limit
	if start >= len(imageIDs) {
		// Si la página solicitada está fuera del rango de IDs disponibles
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{})
		return
	}
	if end > len(imageIDs) {
		end = len(imageIDs)
	}
	paginatedImageIDs := imageIDs[start:end]

	images, err := services.BuscarImagenes(w, paginatedImageIDs, db)
	if err != nil {
		http.Error(w, "Error al buscar imágenes", http.StatusInternalServerError)
		return
	}

	// Devolver la lista de URLs de las miniaturas
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

func ImportarHandler(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket, database *mongo.Database) {
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
	datos, err := services.ProcesarDonacionFisica(w, r)
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
	}
	//Subir informacion e imagenes a mongo
	services.SubirDonacionFisica(datos, w, bucket, r, database)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"message": "Data successfully inserted"}
	json.NewEncoder(w).Encode(response)
}

// CreateDiagnosticoHandler maneja la solicitud para crear un diagnóstico
func CreateDiagnosticoHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	var diagnostico models.Diagnostico
	err := json.NewDecoder(r.Body).Decode(&diagnostico)
	if err != nil {
		http.Error(w, "Error en la solicitud: "+err.Error(), http.StatusBadRequest)
		log.Printf("Error al decodificar el diagnóstico: %s", err.Error())
		return
	}

	estudioID := r.URL.Query().Get("estudioID") // Obtén el estudioID de la URL
	log.Printf("Recibiendo solicitud para crear diagnóstico para estudio_ID: %s", estudioID)

	// Llama a la función para guardar el diagnóstico
	err = services.CreateDiagnostico(db, estudioID, diagnostico)
	if err != nil {
		http.Error(w, "Error al guardar el diagnóstico: "+err.Error(), http.StatusInternalServerError)
		log.Printf("Error al guardar diagnóstico: %s", err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode("Diagnóstico guardado exitosamente")
}
