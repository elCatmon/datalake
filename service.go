package main

//Codigo generado por Cesar Ortega

// Importacion de librerias
import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/disintegration/imaging"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"golang.org/x/crypto/bcrypt"
)

// Usuarios
// Registra nuevas cuentas de usuario
func RegistrarUsuario(db *sql.DB, user User) error {
	query := `INSERT INTO users (nombre, correo, contrasena) VALUES ($1, $2, $3)`

	log.Printf("Ejecutando consulta: %s", query)
	_, err := db.Exec(query, user.Nombre, user.Correo, user.Contrasena)
	if err != nil {
		return fmt.Errorf("error al registrar usuario: %v", err)
	}

	return nil
}

// Valida que no existe un correo ya registrado al momento de crear una cuenta
func ExisteCorreo(db *sql.DB, email string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM users WHERE correo=$1)"

	log.Printf("Ejecutando consulta: %s", query)
	err := db.QueryRow(query, email).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// ValidarUsuario verifica las credenciales del usuario y devuelve el ID del usuario si son válidas.
func ValidarUsuario(db *sql.DB, correo, contrasena string) (bool, string, error) {
	var id string
	var storedPassword string

	// Consulta para obtener la contraseña almacenada y el ID del usuario
	err := db.QueryRow("SELECT usuario_id, contrasena FROM users WHERE correo = $1", correo).Scan(&id, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			// Usuario no encontrado
			return false, "", nil
		}
		// Otro error
		return false, "", err
	}

	// Verificar la contraseña usando bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(contrasena))
	if err != nil {
		// Contraseña incorrecta
		return false, "", nil
	}

	return true, id, nil
}

// HashPassword genera el hash de una contraseña utilizando bcrypt.
func HashContraseña(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

//Recuperar contraseña

// Busqueda de estudios
// EncontrarImagen busca una imagen en GridFS por nombre de archivo.
func EncontrarImagen(bucket *gridfs.Bucket, filename string) (*gridfs.DownloadStream, error) {
	downloadStream, err := bucket.OpenDownloadStreamByName(filename)
	if err != nil {
		return nil, err
	}
	return downloadStream, nil
}

// Crear filtro de busqueda de estudios
func CrearFiltro(w http.ResponseWriter, tipoEstudio string, region string, edadMin string, edadMax string, sexo string) (bson.M, error) {
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
	if edadMin != "" || edadMax != "" {
		edadFilter := bson.M{}
		if edadMin != "" {
			edadMinInt, err := strconv.Atoi(edadMin)
			if err != nil {
				http.Error(w, "Edad mínima inválida", http.StatusBadRequest)
			}
			edadFilter["$gte"] = edadMinInt
		}
		if edadMax != "" {
			edadMaxInt, err := strconv.Atoi(edadMax)
			if err != nil {
				http.Error(w, "Edad máxima inválida", http.StatusBadRequest)
			}
			edadFilter["$lte"] = edadMaxInt
		}
		filter["edad"] = edadFilter
	}
	if sexo != "" {
		filter["sexo"] = sexo
	}
	return filter, nil
}

// Busca estudios aplicando el filtro
func buscarEstudios(w http.ResponseWriter, studiesCollection *mongo.Collection, filter bson.M) ([]primitive.ObjectID, *mongo.Cursor, error) {

	// Buscar los estudios que cumplen con el filtro
	cursor, err := studiesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar estudios en la base de datos: "+err.Error(), http.StatusInternalServerError)
	}
	defer cursor.Close(context.Background())

	// Recolectar IDs de imágenes que cumplen con los criterios
	var imageIDs []primitive.ObjectID
	for cursor.Next(context.Background()) {
		var study EstudioDocument
		if err := cursor.Decode(&study); err != nil {
			http.Error(w, "Error al decodificar estudio: "+err.Error(), http.StatusInternalServerError)
		}

		for _, img := range study.Imagenes {
			if img.Anonimizada {
				imageID, err := primitive.ObjectIDFromHex(img.Imagen)
				if err != nil {
					http.Error(w, "Error al convertir ID de imagen: "+err.Error(), http.StatusInternalServerError)
				}
				imageIDs = append(imageIDs, imageID)
			}
		}
	}

	return imageIDs, cursor, nil
}

// Encuentra y regresa miniaturas de los estudios
func BuscarImagenes(w http.ResponseWriter, imageIDs []primitive.ObjectID, db *mongo.Database) ([]string, error) {
	// Obtener la colección de archivos GridFS
	imagesCollection := db.Collection("imagenes.files")
	// Filtrar archivos con IDs en imageIDs y que terminen en .jpg
	filter := bson.M{
		"_id":      bson.M{"$in": imageIDs},
		"filename": bson.M{"$regex": `\.jpg$`},
	}
	// Buscar los archivos en la colección usando el filtro
	cursor, err := imagesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar archivos en la base de datos: "+err.Error(), http.StatusInternalServerError)
	}
	defer cursor.Close(context.Background())
	var images []string
	for cursor.Next(context.Background()) {
		var fileInfo FileDocument
		if err := cursor.Decode(&fileInfo); err != nil {
			http.Error(w, "Error al decodificar archivo: "+err.Error(), http.StatusInternalServerError)
		}
		// Obtener el nombre del archivo y construir la URL
		filename := fileInfo.Filename
		if filename != "" {
			imageURL := ip + "/image/" + filename
			images = append(images, imageURL)
		}
	}
	return images, nil
}

// Donacion de estudios
// Procesamiento de los datos de donacion fisica
func ProcesarDonacionFisica(w http.ResponseWriter, r *http.Request) ([]interface{}, error) {

	formData := r.MultipartForm

	// Verificar campos obligatorios
	estudioID, err := getValueOrError(formData.Value, "estudio_ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	estudio, err := getValueOrError(formData.Value, "estudio")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	region, err := getValueOrError(formData.Value, "region")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	valida, err := getValueOrError(formData.Value, "imagenValida")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	sexo, err := getValueOrError(formData.Value, "sexo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	edad, err := getValueOrError(formData.Value, "edad")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	proyeccion, err := getValueOrError(formData.Value, "proyeccion")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	donador, err := getValueOrError(formData.Value, "donador")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	numeroOperacion, err := getValueOrError(formData.Value, "estudio_ID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	// Generar el hash a partir del nombre del donador y el número de operación
	hash := generateHash(donador, numeroOperacion)

	// Procesamiento de archivos originales
	originalFiles := formData.File["archivosOriginales"]
	anonymizedFiles := formData.File["archivosAnonimizados"]

	if len(originalFiles) == 0 {
		http.Error(w, "No original files uploaded", http.StatusBadRequest)
		return nil, errors.New("no original files uploaded")
	}

	if len(anonymizedFiles) == 0 {
		http.Error(w, "No anonymized files uploaded", http.StatusBadRequest)
		return nil, errors.New("no anonymized files uploaded")
	}

	datos := []interface{}{estudioID, donador, estudio, hash, region, valida, sexo, edad, proyeccion, anonymizedFiles, originalFiles}

	return datos, err
}

// Función para generar un hash SHA-256
func generateHash(donador, numOperacion string) string {
	hashInput := donador + numOperacion
	hash := sha256.New()
	hash.Write([]byte(hashInput))
	return hex.EncodeToString(hash.Sum(nil))
}

// Sube informacion de la donacion fisica anonimizada
func subirDonacionFisica(datos []interface{}, w http.ResponseWriter, bucket *gridfs.Bucket, r *http.Request, database *mongo.Database) {
	estudioID, _ := datos[0].(string)
	donador, _ := datos[1].(string)
	estudio, _ := datos[2].(string)
	hash, _ := datos[3].(string)
	region, _ := datos[4].(string)
	valida, _ := datos[5].(string)
	sexo, _ := datos[6].(string)
	edad, _ := datos[7].(string)
	proyeccion, _ := datos[8].(string)
	anonymizedFiles, _ := datos[9].([]*multipart.FileHeader)
	originalFiles, _ := datos[10].([]*multipart.FileHeader)
	log.Print(estudio)
	log.Print(valida)
	log.Print(region)
	log.Print(proyeccion)
	log.Print(sexo)
	log.Print(edad)
	clave := estudio + "0" + "1" + valida + region + "00" + sexo + edad
	log.Print(clave)
	// Verificar la longitud de los slices antes de usarlos
	if len(originalFiles) == 0 {
		http.Error(w, "No hay archivos originales", http.StatusBadRequest)
		return
	}
	if len(anonymizedFiles) == 0 {
		http.Error(w, "No hay archivos anonimizados", http.StatusBadRequest)
		return
	}

	var imagenes []Imagen

	// Subir archivos originales
	for _, fileHeader := range originalFiles {
		log.Printf("Procesando archivos originales: %s", fileHeader.Filename)
		fileID, err := subirArchivoGridFS(fileHeader, bucket)
		if err != nil {
			http.Error(w, "Fallo al subir a la base de datos los archivos originales", http.StatusInternalServerError)
			return
		}

		imagenes = append(imagenes, Imagen{
			Clave:       clave,
			Dicom:       fileID,
			Imagen:      fileID,
			Anonimizada: false,
		})
	}

	// Subir archivos anonimizados
	for _, fileHeader := range anonymizedFiles {
		fileID, err := subirArchivoGridFS(fileHeader, bucket)
		if err != nil {
			http.Error(w, "Fallo al subir a la base de datos los archivos anonimizados", http.StatusInternalServerError)
			return
		}
		imagenes = append(imagenes, Imagen{
			Clave:       clave,
			Dicom:       fileID,
			Imagen:      fileID,
			Anonimizada: true,
		})
	}

	// Crear el documento del estudio
	estudioDoc := EstudioDocument{
		EstudioID: estudioID,
		Donador:   donador,
		Hash:      hash, // Asignar el hash generado
		Status:    0,
		Imagenes:  imagenes,
		Diagnostico: []Diagnostico{
			{
				Hallazgos:     "",
				Impresion:     "",
				Observaciones: "",
			},
		},
	}

	// Insertar el documento en MongoDB
	collection := database.Collection("estudios")
	_, err := collection.InsertOne(r.Context(), estudioDoc)
	if err != nil {
		http.Error(w, "Failed to insert document", http.StatusInternalServerError)
	}
}

// Función para subir archivos a GridFS
func subirArchivoGridFS(fileHeader *multipart.FileHeader, bucket *gridfs.Bucket) (string, error) {
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

// CreateDiagnostico guarda un diagnóstico en el estudio correspondiente
func CreateDiagnostico(db *mongo.Database, estudioID string, diagnostico Diagnostico) error {
	collection := db.Collection("estudios")

	// Encuentra el documento correspondiente al estudioID
	filter := bson.M{"estudio_ID": estudioID} // Usamos estudio_ID para filtrar
	log.Printf("Buscando estudio con estudio_ID: %s", estudioID)

	// Prepara la actualización para agregar el nuevo diagnóstico
	update := bson.M{
		"$push": bson.M{"diagnostico": diagnostico}, // Agregar al array de diagnósticos
	}

	// Ejecuta la actualización
	result, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Printf("Error al actualizar el diagnóstico: %s", err.Error())
		return err
	}

	if result.ModifiedCount == 0 {
		log.Printf("No se modificó ningún documento para el estudio_ID: %s", estudioID)
		return fmt.Errorf("no se modificó ningún documento")
	}

	log.Printf("Diagnóstico guardado exitosamente para estudio_ID: %s", estudioID)
	return nil
}

func SubirDonacionDigital(w http.ResponseWriter, bucket *gridfs.Bucket, r *http.Request, database *mongo.Database) error {
	log.Println("Iniciando procesamiento de donación digital")
	err := r.ParseMultipartForm(10 << 20) // Límite de 10MB por archivo
	if err != nil {
		log.Printf("Error al parsear el formulario: %v", err)
		http.Error(w, "Error al procesar los archivos", http.StatusBadRequest)
		return err
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		log.Println("No se proporcionaron archivos")
		http.Error(w, "Debe proporcionar al menos un archivo", http.StatusBadRequest)
		return fmt.Errorf("no se proporcionaron archivos")
	}

	var imagenes []Imagen
	var anonymizedFiles []string
	var jpgFiles []string

	// Iterar sobre cada archivo enviado
	for _, fileHeader := range files {
		log.Printf("Procesando archivo: %s", fileHeader.Filename)

		// Abrir el archivo subido
		file, err := fileHeader.Open()
		if err != nil {
			log.Printf("Error al abrir el archivo %s: %v", fileHeader.Filename, err)
			http.Error(w, "Error al abrir el archivo", http.StatusBadRequest)
			continue // Continuar con el siguiente archivo
		}
		defer file.Close()

		// Guardar el archivo temporalmente
		tempFilePath := "./archivos/" + fileHeader.Filename
		log.Printf("Guardando archivo temporal: %s", tempFilePath)
		tempFile, err := os.Create(tempFilePath)
		if err != nil {
			log.Printf("Error al crear el archivo temporal %s: %v", tempFilePath, err)
			http.Error(w, "Error al crear el archivo temporal", http.StatusInternalServerError)
			continue
		}
		defer tempFile.Close()

		_, err = io.Copy(tempFile, file)
		if err != nil {
			log.Printf("Error al copiar el archivo %s: %v", fileHeader.Filename, err)
			http.Error(w, "Error al copiar el archivo", http.StatusInternalServerError)
			continue
		}

		// Anonimizar el archivo
		anonFilePath := tempFilePath + "_M.dcm"
		log.Printf("Anonimizando archivo %s a %s", tempFilePath, anonFilePath)
		err = anonimizarArchivo(tempFilePath, anonFilePath)
		if err != nil {
			log.Printf("Error al anonimizar el archivo %s: %v", tempFilePath, err)
			http.Error(w, "Error al anonimizar el archivo", http.StatusInternalServerError)
			continue
		}

		jpgtempFilePath := filepath.Ext(anonFilePath) + ".jpg"
		err = convertirArchivo(anonFilePath, jpgtempFilePath)
		if err != nil {
			log.Printf("Error al convertir el archivo %s: %v", tempFilePath, err)
			http.Error(w, "Error al convertir el archivo", http.StatusInternalServerError)
			continue
		}

		// Guardar rutas de archivos anonimizados y JPG
		anonymizedFiles = append(anonymizedFiles, anonFilePath)
		jpgFiles = append(jpgFiles, jpgtempFilePath)

		// Limpieza de archivos temporales
		log.Printf("Eliminando archivo temporal %s", tempFilePath)
		os.Remove(tempFilePath)
	}

	// Crear una clave única para los archivos
	estudioID := primitive.NewObjectID().Hex()
	donador := "DonadorEjemplo" // Valor ejemplo, reemplazar por el valor correcto
	estudio, err := getValueOrError(r.MultipartForm.Value, "tipoEstudio")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	hash := generateHash(donador, estudio)
	clave := estudio + "0" + "1" + "0" + "00" + "00" + "0" + "0"
	log.Printf("Generando documento de estudio con ID: %s", estudioID)

	// Subir archivos originales a GridFS y crear el documento del estudio
	for _, fileHeader := range files {
		log.Printf("Subiendo archivo original %s a GridFS", fileHeader.Filename)
		fileID := subirArchivoDigitalGridFS("./archivos/"+fileHeader.Filename, bucket)
		if fileID == "" {
			log.Printf("Fallo al subir el archivo original %s", fileHeader.Filename)
			http.Error(w, "Fallo al subir a la base de datos los archivos originales", http.StatusInternalServerError)
			continue
		}
		imagenes = append(imagenes, Imagen{
			Clave:       clave,
			Dicom:       fileID,
			Imagen:      "",
			Anonimizada: false,
		})
	}

	// Subir archivos anonimizados y JPG a GridFS
	for i := range anonymizedFiles {
		log.Printf("Subiendo archivo anonimizado %s a GridFS", anonymizedFiles[i])
		fileID := subirArchivoDigitalGridFS(anonymizedFiles[i], bucket)
		jpgID := subirArchivoDigitalGridFS(jpgFiles[i], bucket)

		if fileID == "" || jpgID == "" {
			log.Printf("Fallo al subir los archivos %s o %s", anonymizedFiles[i], jpgFiles[i])
			http.Error(w, "Fallo al subir a la base de datos los archivos anonimizados", http.StatusInternalServerError)
			continue
		}
		imagenes = append(imagenes, Imagen{
			Clave:       clave,
			Dicom:       fileID,
			Imagen:      jpgID,
			Anonimizada: true,
		})
	}

	// Crear el documento del estudio
	estudioDoc := EstudioDocument{
		EstudioID: estudioID,
		Donador:   donador,
		Hash:      hash,
		Status:    0,
		Imagenes:  imagenes,
		Diagnostico: []Diagnostico{
			{
				Hallazgos:     "",
				Impresion:     "",
				Observaciones: "",
			},
		},
	}
	log.Println("Insertando el documento de estudio en MongoDB")

	// Insertar el documento en MongoDB
	collection := database.Collection("estudios")
	_, err = collection.InsertOne(context.Background(), estudioDoc)
	if err != nil {
		log.Printf("Fallo al insertar el documento de estudio: %v", err)
		http.Error(w, "Fallo al insertar el documento", http.StatusInternalServerError)
		return err
	}

	// Respuesta de éxito
	log.Println("Todos los archivos han sido procesados exitosamente y el estudio ha sido registrado.")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Todos los archivos han sido procesados exitosamente y el estudio ha sido registrado."))

	return nil
}

// Función para subir archivos a GridFS
func subirArchivoDigitalGridFS(filePath string, bucket *gridfs.Bucket) string {
	log.Printf("Subiendo archivo a GridFS: %s", filePath)

	// Abrir el archivo para subir a GridFS
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error al abrir archivo para subir a GridFS: %s, error: %v", filePath, err)
		return ""
	}
	defer file.Close()

	// Crear el archivo en GridFS
	uploadStream, err := bucket.OpenUploadStream(filepath.Base(filePath))
	if err != nil {
		log.Printf("Error al abrir flujo de subida en GridFS para %s: %v", filePath, err)
		return ""
	}
	defer uploadStream.Close()

	// Copiar el contenido del archivo al flujo de subida
	_, err = io.Copy(uploadStream, file)
	if err != nil {
		log.Printf("Error al copiar archivo al flujo de subida en GridFS: %s, error: %v", filePath, err)
		return ""
	}

	// Obtener el ID del archivo subido
	fileID := uploadStream.FileID.(primitive.ObjectID)
	log.Printf("Archivo subido a GridFS con ID: %s", fileID.Hex())
	return fileID.Hex()
}

// Función para ejecutar el script de anonimización
func anonimizarArchivo(tempFilePath, anonFilePath string) error {
	log.Printf("Ejecutando script de anonimización para %s", tempFilePath)
	var cmd *exec.Cmd

	// Detectar el sistema operativo
	if runtime.GOOS == "windows" {
		cmd = exec.Command("python", "./services/anonimizacion.py", tempFilePath, anonFilePath)
	} else {
		cmd = exec.Command("python3", "./services/anonimizacion.py", tempFilePath, anonFilePath)
	}

	// Ejecutar el comando
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error al ejecutar el script de anonimización: %v, output: %s", err, string(output))
		return fmt.Errorf("error al ejecutar el script de anonimización: %w", err)
	}

	log.Printf("Script de anonimización ejecutado correctamente para %s", tempFilePath)
	return nil
}

func convertirArchivo(tempFilePath, jpgFilePath string) error {
	log.Printf("Ejecutando script de anonimización para %s", tempFilePath)
	var cmd *exec.Cmd

	// Detectar el sistema operativo
	if runtime.GOOS == "windows" {
		cmd = exec.Command("python", "./services/dcm_jpg.py", tempFilePath, jpgFilePath)
	} else {
		cmd = exec.Command("python3", "./services/dcm_jpg.py", tempFilePath, jpgFilePath)
	}

	// Ejecutar el comando
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error al ejecutar el script de anonimización: %v, output: %s", err, string(output))
		return fmt.Errorf("error al ejecutar el script de anonimización: %w", err)
	}

	log.Printf("Script de anonimización ejecutado correctamente para %s", tempFilePath)

	return nil
}

// Conversion de archivos

// Descarga conjunto de datos
