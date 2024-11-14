package Handl

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"io"
	"net/http"
	"strconv"

	"webservice/models"
	"webservice/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"golang.org/x/crypto/bcrypt"
)

// RegisterHandler maneja la solicitud de registro de un nuevo usuario.
func RegisterHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	w.Header().Set("Content-Type", "application/json")
	var newUser models.User
	err := json.NewDecoder(r.Body).Decode(&newUser)
	if err != nil {
		http.Error(w, `{"error": "Error al decodificar los datos"}`, http.StatusBadRequest)
		return
	}
	exists, err := services.ExisteCorreo(db, newUser.Correo)
	if err != nil {
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
		http.Error(w, `{"error": "Error al encriptar la contraseña"}`, http.StatusInternalServerError)
		return
	}

	newUser.Contrasena = string(hashedPassword)
	err = services.RegistrarUsuario(db, newUser)
	if err != nil {
		http.Error(w, `{"error": "Error al registrar usuario"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{"message": "Registro exitoso"}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// LoginHandler maneja la solicitud de inicio de sesión del usuario.
func LoginHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	var credentials models.User
	err := json.NewDecoder(r.Body).Decode(&credentials)
	if err != nil {
		http.Error(w, `{"error": "Error al decodificar los datos"}`, http.StatusBadRequest)
		return
	}

	isValid, id, curp, rol, authErr := services.ValidarUsuario(db, credentials.Correo, credentials.Contrasena)
	if authErr != nil {
		http.Error(w, `{"error": "Error interno del servidor"}`, http.StatusInternalServerError)
		return
	}

	if !isValid {
		http.Error(w, `{"error": "Correo o contraseña incorrectos"}`, http.StatusUnauthorized)
		return
	}

	// Incluir el rol en la respuesta
	response := map[string]string{
		"message": "Inicio de sesión exitoso",
		"id":      id,
		"curp":    curp,
		"rol":     rol,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, `{"error": "Error al codificar la respuesta"}`, http.StatusInternalServerError)
	}
}

// Manejador para verificar si existe un usuario con el correo proporcionado
func VerificarCorreoHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	var req struct {
		Email string `json:"email"`
	}

	// Decodificar el cuerpo de la solicitud
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error en la solicitud", http.StatusBadRequest)
		return
	}

	// Verificar si el correo existe en la base de datos
	exists, err := services.ExisteCorreo(db, req.Email)
	if err != nil {
		http.Error(w, "Error al verificar el correo", http.StatusInternalServerError)
		return
	}

	// Devolver la respuesta en formato JSON
	json.NewEncoder(w).Encode(map[string]bool{"exists": exists})
}

// Manejador para cambiar la contraseña
func CambiarContrasenaHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	var req struct {
		Email           string `json:"email"`
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	// Decodificar la solicitud
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error en la solicitud", http.StatusBadRequest)
		return
	}

	// Intentar cambiar la contraseña
	err := services.ChangePassword(db, req.Email, req.CurrentPassword, req.NewPassword)
	if err != nil {
		http.Error(w, "Error al cambiar la contraseña: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respuesta de éxito
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Contraseña cambiada con éxito"})
}

func UploadHandler(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket, database *mongo.Database) {
	// Ejecutar la función de procesamiento
	err := services.SubirDonacionDigital(w, bucket, r, database)

	// Si ocurrió un error en SubirDonacionDigital, no es necesario llamar a WriteHeader ni Write de nuevo
	if err != nil {
		return // La respuesta de error ya ha sido enviada dentro de SubirDonacionDigital
	}
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

func ThumbnailHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	// Obtener la colección de estudios
	studiesCollection := db.Collection("estudios")

	// Obtener parámetros de consulta
	queryParams := r.URL.Query()

	// Parámetros de paginación
	pageStr := queryParams.Get("page")
	limitStr := queryParams.Get("limit")

	// Parsear parámetros de página
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1 // Página por defecto si el parámetro es inválido
	}

	// Parsear parámetros de límite
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 18 // Límite por defecto si el parámetro es inválido
	}

	// Crear filtro para los estudios
	filter, err := services.CrearFiltro(w, r)
	if err != nil {
		http.Error(w, "Error al crear el filtro: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Buscar estudios
	imageIDs, cursor, err := services.BuscarEstudios(w, studiesCollection, filter)
	if err != nil {
		return // Al hacer http.Error, simplemente retornamos
	}
	defer cursor.Close(context.Background()) // Cerrar el cursor después de su uso

	// Si no hay IDs de imagen, devolver una lista vacía
	if len(imageIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{}) // No es necesario usar http.Error aquí
		return
	}

	// Aplicar paginación
	start := (page - 1) * limit
	end := start + limit
	if start >= len(imageIDs) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{}) // No es necesario usar http.Error aquí
		return
	}
	if end > len(imageIDs) {
		end = len(imageIDs)
	}
	paginatedImageIDs := imageIDs[start:end]

	// Buscar imágenes
	images, err := services.BuscarImagenes(w, paginatedImageIDs, db)
	if err != nil {
		return // Al hacer http.Error, simplemente retornamos
	}

	// Devolver la lista de URLs de las miniaturas
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(images); err != nil {
		http.Error(w, "Error al escribir la respuesta: "+err.Error(), http.StatusInternalServerError)
	}
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

func ActualizarDiagnosticoHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	vars := mux.Vars(r)
	studyID := vars["id"]

	fmt.Println("ID del estudio recibido:", studyID)

	var requestBody struct {
		ImagenNombre string             `json:"imagenNombre"` // Cambia la estructura para incluir imagenNombre
		Clave        string             `json:"clave"`        // Cambia la estructura para incluir clave
		Diagnostico  models.Diagnostico `json:"diagnostico"`
	}

	// Decodificar el cuerpo de la solicitud
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		fmt.Println("Error al decodificar los datos del diagnóstico:", err)
		http.Error(w, "Error al decodificar los datos del diagnóstico", http.StatusBadRequest)
		return
	}

	// Asignar la fecha actual si no se proporciona
	requestBody.Diagnostico.Fecha = time.Now()

	// Obtener el nombre de la imagen y la nueva clave
	imagenNombre := requestBody.ImagenNombre
	nuevaClave := requestBody.Clave

	fmt.Println("Nombre de la imagen recibido:", imagenNombre)
	fmt.Println("Nueva clave recibida:", nuevaClave)
	fmt.Println("Datos del diagnóstico recibidos:", requestBody.Diagnostico)

	// Llamar al servicio para actualizar el diagnóstico y la clave de la imagen
	fmt.Println("Intentando actualizar el diagnóstico en la base de datos...")
	err = services.ActualizarDiagnosticoYClave(studyID, imagenNombre, requestBody.Diagnostico, nuevaClave, db)
	if err != nil {
		fmt.Println("Error al actualizar el diagnóstico:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println("Diagnóstico actualizado exitosamente")
	json.NewEncoder(w).Encode(bson.M{"message": "Diagnóstico actualizado exitosamente"})
}

// FindEstudioIDByImagenNombreHandler maneja la búsqueda del _id del estudio que contiene una imagen por su nombre.
func BuscarEstudioIDImagenNombreHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	imagenNombre := r.URL.Query().Get("nombre")
	if imagenNombre == "" {
		http.Error(w, "El nombre de la imagen es requerido", http.StatusBadRequest)
		return
	}
	estudioID, err := services.BuscarEstudioIDImagen(imagenNombre, db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Retornar el _id del estudio como respuesta JSON
	response := struct {
		EstudioID primitive.ObjectID `json:"estudio_id"`
	}{EstudioID: estudioID}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetDiagnosticoHandler maneja la solicitud HTTP para obtener el diagnóstico más reciente
func GetDiagnosticoHandler(w http.ResponseWriter, r *http.Request, db *mongo.Database) {
	// Obtener el ID desde los parámetros de la URL
	idParam := r.URL.Query().Get("id")
	if idParam == "" {
		http.Error(w, "Falta el parámetro 'id'", http.StatusBadRequest)
		return
	}

	nombreImagen := r.URL.Query().Get("nombreImagen")
	if nombreImagen == "" {
		http.Error(w, "Falta el parámetro 'nombreImagen'", http.StatusBadRequest)
		return
	}

	// Convertir el id a ObjectID
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	// Llamar al servicio para buscar el diagnóstico más reciente y la clave de la imagen
	ctx := r.Context()
	diagnosticoReciente, clave, err := services.BuscarDiagnosticoReciente(ctx, db, id, nombreImagen)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Crear la respuesta con diagnóstico y clave
	response := struct {
		Diagnostico *models.Diagnostico `json:"diagnostico"`
		Clave       string              `json:"clave"`
	}{
		Diagnostico: diagnosticoReciente,
		Clave:       clave,
	}

	// Responder con el diagnóstico más reciente y la clave en formato JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func DatasetHandler(w http.ResponseWriter, r *http.Request, bucket *gridfs.Bucket, db *mongo.Database) {
	// Inicializa tus variables necesarias
	collection := db.Collection("estudios")
	var estudios []models.EstudioDocument

	// Buscar todos los estudios
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		http.Error(w, "Error buscando estudios", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	if err := cursor.All(context.Background(), &estudios); err != nil {
		http.Error(w, "Error al convertir cursor a lista", http.StatusInternalServerError)
		return
	}

	tipoArchivo := r.URL.Query().Get("type") // O "jpg", dependiendo de lo que necesites

	// Configura las cabeceras para la descarga
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=dataset_%s_%s.zip", tipoArchivo, time.Now().Format("20060102_150405")))

	// Crea un nuevo zip writer para enviar el contenido al cliente
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Llama a la función RenombrarArchivosZip para generar el ZIP
	if err := services.GenerarDataset(estudios, bucket, zipWriter, tipoArchivo); err != nil {
		http.Error(w, "Error al generar el ZIP", http.StatusInternalServerError)
		return
	}
}

// DescargarDatasetHandler es el handler que maneja la descarga del dataset
func DatasetPredeterminadoHandler(w http.ResponseWriter, r *http.Request) {
	// Llamar al servicio que retorna la ruta del dataset generado
	datasetPath := "./dataset/dataset_dcm_2024-10.zip"

	// Extraer el nombre del archivo a partir de la ruta
	fileName := filepath.Base(datasetPath)

	// Abrir el archivo para lectura
	file, err := os.Open(datasetPath)
	if err != nil {
		http.Error(w, "No se pudo abrir el archivo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Configurar los headers de la respuesta para descarga de archivo
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", "application/octet-stream")

	// Copiar el contenido del archivo al ResponseWriter
	http.ServeFile(w, r, datasetPath)
}
