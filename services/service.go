package services

//Codigo generado por Cesar Ortega

// Importacion de librerias
import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"webservice/config"
	"webservice/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"golang.org/x/crypto/bcrypt"
)

// Usuarios
// Registra nuevas cuentas de usuario
func RegistrarUsuario(db *mongo.Database, user models.User) error {
	collection := db.Collection("usuarios")

	// Log de inicio del registro
	log.Printf("Intentando registrar usuario con correo: %s", user.Correo)

	// Insertar el nuevo usuario en MongoDB
	_, err := collection.InsertOne(context.TODO(), bson.M{
		"nombre":     user.Nombre,
		"correo":     user.Correo,
		"contrasena": user.Contrasena,
		"rol":        "consultor", // Asigna un rol al usuario
	})
	if err != nil {
		log.Printf("Error al registrar usuario con correo %s: %v", user.Correo, err)
		return fmt.Errorf("error al registrar usuario: %v", err)
	}

	// Log de éxito
	log.Printf("Usuario registrado exitosamente con correo: %s", user.Correo)
	return nil
}

// Valida que no existe un correo ya registrado al momento de crear una cuenta
func ExisteCorreo(db *mongo.Database, email string) (bool, error) {
	collection := db.Collection("usuarios")

	// Log de inicio de verificación
	log.Printf("Verificando existencia de correo: %s", email)

	// Buscar si existe un usuario con el correo proporcionado
	count, err := collection.CountDocuments(context.TODO(), bson.M{"correo": email})
	if err != nil {
		log.Printf("Error al verificar correo %s: %v", email, err)
		return false, fmt.Errorf("error al verificar el correo: %v", err)
	}

	// Log de resultado de la verificación
	if count > 0 {
		log.Printf("El correo %s ya está registrado", email)
	} else {
		log.Printf("El correo %s no está registrado", email)
	}

	// Si `count` es mayor que 0, significa que el correo ya existe
	return count > 0, nil
}

// ValidarUsuario verifica las credenciales del usuario y devuelve el ID del usuario si son válidas.
func ValidarUsuario(db *mongo.Database, correo, contrasena string) (bool, string, error) {
	collection := db.Collection("usuarios")
	var user models.User

	// Log de inicio de validación
	log.Printf("Validando credenciales para el correo: %s", correo)

	// Buscar al usuario por correo
	err := collection.FindOne(context.TODO(), bson.M{"correo": correo}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Usuario no encontrado
			log.Printf("Usuario no encontrado con el correo: %s", correo)
			return false, "", nil
		}
		log.Printf("Error al buscar usuario con correo %s: %v", correo, err)
		return false, "", fmt.Errorf("error al buscar usuario: %v", err)
	}

	// Verificar la contraseña usando bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(user.Contrasena), []byte(contrasena))
	if err != nil {
		// Contraseña incorrecta
		log.Printf("Contraseña incorrecta para el correo: %s", correo)
		return false, "", nil
	}

	// Si todo es correcto, devuelve true y el ID del usuario
	log.Printf("Credenciales válidas para el usuario con correo: %s", correo)
	return true, user.ID.Hex(), nil
}

// generadores
// HashPassword genera el hash de una contraseña utilizando bcrypt.

// Función para generar un hash SHA-256
func GenerateHash(donador, numOperacion string) string {
	hashInput := donador + numOperacion
	hash := sha256.New()
	hash.Write([]byte(hashInput))
	return hex.EncodeToString(hash.Sum(nil))
}

// Busqueda de errores
// Función para obtener valores del formulario o devolver un error si el campo no existe
func getValueOrError(formData map[string][]string, key string) (string, error) {
	values, ok := formData[key]
	if !ok || len(values) == 0 {
		return "", errors.New("Missing or empty field: " + key)
	}
	return values[0], nil
}

//Recuperar contraseña

// Busqueda de estudios
// EncontrarImagen busca una imagen en GridFS por nombre de archivo.
func EncontrarImagen(bucket *gridfs.Bucket, filename string) (*gridfs.DownloadStream, error) {
	downloadStream, err := bucket.OpenDownloadStreamByName(filename)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el stream de descarga para el archivo %s: %w", filename, err)
	}
	return downloadStream, nil
}

// Crear filtro de búsqueda de estudios basados en la clave personalizada
func CrearFiltro(w http.ResponseWriter, r *http.Request) (bson.M, error) {
	tipoEstudio := r.URL.Query().Get("tipoEstudio")
	region := r.URL.Query().Get("region")
	proyeccion := r.URL.Query().Get("proyeccion")

	// Filtro obligatorio
	if tipoEstudio == "" {
		return nil, fmt.Errorf("el campo tipoEstudio es obligatorio")
	}

	filter := bson.M{
		"imagenes": bson.M{
			"$elemMatch": bson.M{
				"anonimizada": true, // Obligatorio
			},
		},
		"status": 1, // Obligatorio
		"imagenes.clave": bson.M{
			"$regex": "^" + tipoEstudio,
		},
	}

	// Agregar filtros opcionales
	if region != "" {
		filter["imagenes.clave"].(bson.M)["$regex"] = "^.{2}" + region
	}
	if proyeccion != "" {
		filter["imagenes.clave"].(bson.M)["$regex"] = "^.{4}" + proyeccion
	}

	return filter, nil
}

// Busca estudios aplicando el filtro
func BuscarEstudios(w http.ResponseWriter, studiesCollection *mongo.Collection, filter bson.M) ([]primitive.ObjectID, error) {
	cursor, err := studiesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar estudios: "+err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	defer cursor.Close(context.Background())

	var imageIDs []primitive.ObjectID

	for cursor.Next(context.Background()) {
		var study models.EstudioDocument
		if err := cursor.Decode(&study); err != nil {
			http.Error(w, "Error al decodificar estudio: "+err.Error(), http.StatusInternalServerError)
			return nil, err
		}

		// Buscar las imágenes asociadas al estudio
		imageFilter := bson.M{"estudio_ID": study.EstudioID, "anonimizada": true}
		imageCursor, err := studiesCollection.Database().Collection("imagenes.files").Find(context.Background(), imageFilter)
		if err != nil {
			http.Error(w, "Error al buscar imágenes: "+err.Error(), http.StatusInternalServerError)
			return nil, err
		}
		defer imageCursor.Close(context.Background())

		for imageCursor.Next(context.Background()) {
			var file models.FileDocument
			if err := imageCursor.Decode(&file); err != nil {
				http.Error(w, "Error al decodificar imagen: "+err.Error(), http.StatusInternalServerError)
				return nil, err
			}
			imageIDs = append(imageIDs, file.ID)
		}
	}

	return imageIDs, nil
}

// Encuentra y regresa miniaturas de los estudios
func BuscarImagenes(w http.ResponseWriter, imageIDs []primitive.ObjectID, db *mongo.Database) ([]string, error) {
	imagesCollection := db.Collection("imagenes.files")
	filter := bson.M{
		"_id":      bson.M{"$in": imageIDs},
		"filename": bson.M{"$regex": `\.jpg$`},
	}

	cursor, err := imagesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar archivos: "+err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	defer cursor.Close(context.Background())

	var images []string
	for cursor.Next(context.Background()) {
		var fileInfo models.FileDocument
		if err := cursor.Decode(&fileInfo); err == nil && fileInfo.Filename != "" {
			imageURL := config.GetIP() + "/image/" + fileInfo.Filename
			images = append(images, imageURL)
		} else if err != nil {
			http.Error(w, "Error al decodificar archivo: "+err.Error(), http.StatusInternalServerError)
			return nil, err
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

	sexo, err := getValueOrError(formData.Value, "sexo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	edad, err := getValueOrError(formData.Value, "edad")
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
	hash := GenerateHash(donador, numeroOperacion)

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

	datos := []interface{}{estudioID, donador, estudio, hash, region, "0", sexo, edad, anonymizedFiles, originalFiles}

	return datos, err
}

func SubirDonacionDigital(w http.ResponseWriter, bucket *gridfs.Bucket, r *http.Request, database *mongo.Database) error {
	err := r.ParseMultipartForm(10 << 20) // Límite de 10MB por archivo
	if err != nil {
		http.Error(w, "Error al procesar los archivos", http.StatusBadRequest)
		return err
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "Debe proporcionar al menos un archivo", http.StatusBadRequest)
		return fmt.Errorf("no se proporcionaron archivos")
	}

	var anonymizedFiles []string
	var jpgFiles []string

	// Recopilar valores del formulario
	estudioID := primitive.NewObjectID().Hex() // ID temporal para el estudio
	donador, _ := getValueOrError(r.MultipartForm.Value, "donador")
	estudio, _ := getValueOrError(r.MultipartForm.Value, "tipoEstudio")
	clave := estudio + "00" + "00" + "0" + "0" + "1" + "0" + "0" // Clave generada, ajustar según sea necesario
	hash := GenerateHash(donador, estudio)

	// Crear el documento del estudio
	estudioDoc := models.EstudioDocument{
		ID:          primitive.NewObjectID(),
		EstudioID:   estudioID,
		Donador:     donador,
		Hash:        hash,
		Status:      0,                      // Ajustar según el estado que corresponda
		Diagnostico: []models.Diagnostico{}, // Aquí se pueden agregar diagnósticos si se proporcionan
	}

	// Guardar el estudio en la base de datos
	estudioCollection := database.Collection("estudios")
	_, err = estudioCollection.InsertOne(context.Background(), estudioDoc)
	if err != nil {
		http.Error(w, "Error al guardar el estudio", http.StatusInternalServerError)
		return err
	}

	// Iterar sobre cada archivo enviado
	for _, fileHeader := range files {
		// Abrir el archivo subido
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Error al abrir el archivo", http.StatusBadRequest)
			continue // Continuar con el siguiente archivo
		}
		defer file.Close()

		// Guardar el archivo temporalmente
		tempFilePath := "./archivos/" + fileHeader.Filename
		tempFile, err := os.Create(tempFilePath)
		if err != nil {
			http.Error(w, "Error al crear el archivo temporal", http.StatusInternalServerError)
			continue
		}
		defer tempFile.Close()

		_, err = io.Copy(tempFile, file)
		if err != nil {
			http.Error(w, "Error al copiar el archivo", http.StatusInternalServerError)
			continue
		}

		// Comprobar si el archivo es DICOM
		if filepath.Ext(tempFilePath) == ".dcm" {
			// Anonimizar el archivo
			fileNameWithoutExt := tempFilePath[:len(tempFilePath)-len(filepath.Ext(tempFilePath))]
			anonFilePath := fileNameWithoutExt + "_M.dcm"
			err = anonimizarArchivo(tempFilePath, anonFilePath)
			if err != nil {
				http.Error(w, "Error al anonimizar el archivo", http.StatusInternalServerError)
				continue
			}

			// Convertir el archivo DICOM anonimizado a JPG
			jpgtempFilePath := fileNameWithoutExt + "_M.jpg"
			err = convertirArchivo(anonFilePath, jpgtempFilePath)
			if err != nil {
				http.Error(w, "Error al convertir el archivo a JPG", http.StatusInternalServerError)
				continue
			}

			// Guardar rutas de archivos anonimizados y JPG
			anonymizedFiles = append(anonymizedFiles, anonFilePath)
			jpgFiles = append(jpgFiles, jpgtempFilePath)
		}

		// Subir archivo (ya sea JPG o DICOM) a GridFS
		fileID := subirArchivoDigitalGridFS(tempFilePath, bucket)
		if fileID == "" {
			http.Error(w, "Fallo al subir a la base de datos los archivos originales", http.StatusInternalServerError)
			continue
		}

		// Crear un documento para el archivo
		fileDoc := models.FileDocument{
			ID:          primitive.NewObjectID(),
			Filename:    fileHeader.Filename,
			Length:      fileHeader.Size,
			ChunkSize:   1024 * 255, // Tamaño de chunk por defecto
			UploadDate:  time.Now(),
			EstudioID:   estudioID, // Usar el ID del estudio relacionado
			Anonimizada: true,
			Clave:       clave,
		}

		// Insertar el documento en la colección `imagenes.files`
		_, err = database.Collection("imagenes.files").InsertOne(context.Background(), fileDoc)
		if err != nil {
			http.Error(w, "Error al guardar el documento del archivo", http.StatusInternalServerError)
			continue
		}
	}

	// Responder con éxito
	w.WriteHeader(http.StatusOK)
	return nil
}

// BuscarEstudioIDImagenNombre busca el _id del estudio que contiene una imagen por su nombre.
func BuscarEstudioIDImagen(imagenNombre string, db *mongo.Database) (primitive.ObjectID, error) {
	// Buscando la imagen en la colección de archivos (imagenes.files)
	fileCollection := db.Collection("imagenes.files")
	fileFilter := bson.M{"filename": imagenNombre}
	var fileDoc models.FileDocument
	err := fileCollection.FindOne(context.Background(), fileFilter).Decode(&fileDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return primitive.ObjectID{}, fmt.Errorf("no se encontró la imagen con el nombre: %s", imagenNombre)
		}
		return primitive.ObjectID{}, fmt.Errorf("error al buscar la imagen: %v", err)
	}
	// Buscando el estudio utilizando el ID de la imagen (como cadena de texto)
	studyCollection := db.Collection("estudios")
	studyFilter := bson.M{"imagenes.dicom": fileDoc.ID.Hex()} // Convertir el ObjectID a su representación hexadecimal (cadena)
	var estudio models.EstudioDocument
	err = studyCollection.FindOne(context.Background(), studyFilter).Decode(&estudio)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return primitive.ObjectID{}, fmt.Errorf("no se encontró el estudio que contiene la imagen")
		}
		return primitive.ObjectID{}, fmt.Errorf("error al buscar el estudio: %v", err)
	}
	return estudio.ID, nil
}

// BuscarEstudioIDImagenNombre busca el _id del estudio que contiene una imagen por su nombre para actualizar el diagnostico.
func BuscarImagenEstudioNombre(imagenNombre string, db *mongo.Database) (primitive.ObjectID, error) {
	// Buscando la imagen en la colección de archivos (imagenes.files)
	fileCollection := db.Collection("imagenes.files")
	fileFilter := bson.M{"filename": imagenNombre}
	var fileDoc models.FileDocument
	err := fileCollection.FindOne(context.Background(), fileFilter).Decode(&fileDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return primitive.ObjectID{}, fmt.Errorf("no se encontró la imagen con el nombre: %s", imagenNombre)
		}
		return primitive.ObjectID{}, fmt.Errorf("error al buscar la imagen: %v", err)
	}
	// Devolver el ID de la imagen encontrada
	return fileDoc.ID, nil
}

func ActualizarDiagnosticoYClave(studyID string, imagenNombre string, diagnostico models.Diagnostico, nuevaClave string, db *mongo.Database) error {
	// Convertir el studyID a ObjectID de MongoDB
	objectID, err := primitive.ObjectIDFromHex(studyID)
	if err != nil {
		return fmt.Errorf("ID de estudio inválido: %v", err)
	}

	// Obtener la fecha actual
	fechaActual := time.Now()
	diagnostico.Fecha = fechaActual
	// Buscar el ID de la imagen a partir del nombre
	imagenID, err := BuscarImagenEstudioNombre(imagenNombre, db)
	if err != nil {
		return fmt.Errorf("error al encontrar la imagen: %v", err)
	}

	// Crear el filtro para buscar el estudio por su ID y la imagen específica por su ID
	collection := db.Collection("estudios")
	filter := bson.M{
		"_id":            objectID,
		"imagenes.dicom": imagenID.Hex(), // Filtrar por el ID de la imagen
	}
	// Crear el nuevo diagnóstico para agregar al array
	nuevoDiagnostico := bson.M{
		"hallazgos":     diagnostico.Hallazgos,
		"impresion":     diagnostico.Impresion,
		"observaciones": diagnostico.Observaciones,
		"fecha_Emision": diagnostico.Fecha, // Usar la fecha actual
		"realizo":       diagnostico.Medico,
	}
	// Operación de actualización para agregar el diagnóstico al array y actualizar la clave solo en la imagen seleccionada
	update := bson.M{
		"$push": bson.M{
			"diagnostico": nuevoDiagnostico, // Agregar el nuevo diagnóstico
		},
		"$set": bson.M{
			"imagenes.$.clave": nuevaClave, // Actualizar la clave solo en la imagen seleccionada
		},
	}
	// Ejecutar la actualización en MongoDB
	result, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return fmt.Errorf("error al actualizar el diagnóstico y la clave en la base de datos: %v", err)
	}
	if result.ModifiedCount == 0 {
		return fmt.Errorf("no se encontró el estudio o no se actualizó el diagnóstico y la clave")
	}
	return nil
}

// BuscarDiagnosticoReciente busca el diagnóstico más reciente de un estudio dado su _id y el nombre de la imagen
func BuscarDiagnosticoReciente(ctx context.Context, db *mongo.Database, id primitive.ObjectID, nombreImagen string) (*models.Diagnostico, string, error) {
	// Definir la colección
	collection := db.Collection("estudios")

	// Buscar el documento por _id
	var estudio models.EstudioDocument
	filter := bson.M{"_id": id}
	err := collection.FindOne(ctx, filter).Decode(&estudio)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, "", errors.New("no se encontró el documento con el ID proporcionado")
		}
		return nil, "", err
	}

	// Si no tiene diagnósticos, regresar un error
	if len(estudio.Diagnostico) == 0 {
		return nil, "", errors.New("el estudio no tiene diagnósticos")
	}

	// Encontrar el diagnóstico más reciente
	var diagnosticoReciente models.Diagnostico
	for _, diag := range estudio.Diagnostico {
		if diag.Fecha.After(diagnosticoReciente.Fecha) {
			diagnosticoReciente = diag
		}
	}

	// Obtener la clave de la imagen basada en el nombre de la imagen
	clave, err := GetImageKeyByFileName(nombreImagen, db)
	if err != nil {
		return nil, "", err
	}

	return &diagnosticoReciente, clave, nil
}

// Función para buscar la clave de una imagen por su nombre en GridFS.
func GetImageKeyByFileName(filename string, db *mongo.Database) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Buscar el archivo en GridFS.
	var fileDoc models.FileDocument
	err := db.Collection("imagenes.files").FindOne(ctx, bson.M{"filename": filename}).Decode(&fileDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", errors.New("archivo no encontrado en GridFS")
		}
		return "", fmt.Errorf("error buscando el archivo en GridFS: %v", err)
	}

	// Buscar en la colección `estudios` usando el ID del archivo.
	var estudioDoc models.EstudioDocument
	err = db.Collection("estudios").FindOne(ctx, bson.M{"imagenes.dicom": fileDoc.ID.Hex()}).Decode(&estudioDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", errors.New("no se encontró referencia en la colección de estudios")
		}
		return "", fmt.Errorf("error buscando en la colección de estudios: %v", err)
	}

	// Recorrer las imágenes para encontrar la clave correspondiente.
	for _, imagen := range estudioDoc.Imagenes {
		if imagen.Dicom == fileDoc.ID.Hex() { // Verificar si el DICOM coincide con el ID del archivo en formato hexadecimal.
			return imagen.Clave, nil
		}
	}

	return "", errors.New("clave no encontrada para la imagen")
}

func RenombrarArchivosZip(estudios []models.EstudioDocument, bucket *gridfs.Bucket, rutaZip string, tipoArchivo string) {
	// Crear un nuevo archivo ZIP
	log.Printf("Creando archivo ZIP en la ruta: %s\n", rutaZip)
	outFile, err := os.Create(rutaZip)
	if err != nil {
		log.Fatalf("Error al crear archivo ZIP: %v", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()
	CrearArchivosMetadata()

	// Añadir los archivos de metadata (README.txt fuera de 'metadata', nameconvention.txt dentro)
	for _, file := range []string{"./dataset/README.txt", "./dataset/nameconvention.txt"} {
		fileData, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Error leyendo archivo %s: %v", file, err)
		}

		// Definir la ruta en el ZIP dependiendo del archivo
		var zipPath string
		if file == "./dataset/nameconvention.txt" {
			zipPath = "metadata/" + file // Dentro de 'metadata'
		} else {

			zipPath = "./dataset/" + file // Directamente en la raíz
		}

		// Crear el archivo dentro del ZIP en la ruta correcta
		w, err := zipWriter.Create(zipPath)
		if err != nil {
			log.Fatalf("Error añadiendo %s al ZIP: %v", file, err)
		}

		if _, err := w.Write(fileData); err != nil {
			log.Fatalf("Error escribiendo %s en el ZIP: %v", file, err)
		}
		log.Printf("Archivo %s añadido correctamente al ZIP en la ruta '%s'.\n", file, zipPath)
	}

	serial := 1
	metadatosPorCarpeta := make(map[string][]models.ImagenMetadata) // Map para almacenar los metadatos por carpeta

	// Añadir archivos renombrados al ZIP en la carpeta 'imagenes'
	for _, estudio := range estudios {
		for _, imagen := range estudio.Imagenes {
			if imagen.Anonimizada { // Filtrar solo las imágenes anonimizadas
				var nuevoNombre string

				if tipoArchivo == "dcm" && imagen.Dicom != (primitive.NilObjectID).Hex() {
					nuevoNombre = GenerarNombreArchivo(imagen.Clave, serial) // Solo el nombre base
					pOID, _ := primitive.ObjectIDFromHex(imagen.Dicom)
					// Obtener el archivo DICOM desde GridFS usando el _id almacenado en imagen.Dicom
					archivoDicom, err := ObtenerArchivoDesdeGridFS(bucket, pOID)
					if err != nil {
						log.Printf("Error obteniendo archivo DICOM con ID %v: %v", imagen.Dicom, err)
						continue
					}

					// Obtener los primeros 4 dígitos del nombre del archivo para crear la carpeta
					carpeta := nuevoNombre[:4]

					// Crear el archivo dentro del ZIP con el nuevo nombre en la carpeta correspondiente
					w, err := zipWriter.Create("imagenes/" + carpeta + "/" + nuevoNombre + ".dcm")
					if err != nil {
						log.Fatalf("Error creando archivo %s en el ZIP: %v", nuevoNombre, err)
					}
					if _, err := w.Write(archivoDicom); err != nil {
						log.Fatalf("Error escribiendo archivo %s en el ZIP: %v", nuevoNombre, err)
					}
					log.Printf("Archivo DICOM %s añadido correctamente al ZIP en la carpeta 'imagenes/%s'.\n", nuevoNombre, carpeta)

					// Añadir los metadatos correspondientes al archivo en su carpeta
					if len(estudio.Diagnostico) > 0 {
						diagnosticoReciente := estudio.Diagnostico[len(estudio.Diagnostico)-1]
						metadatosPorCarpeta[carpeta] = append(metadatosPorCarpeta[carpeta], models.ImagenMetadata{
							NombreArchivo: nuevoNombre,
							Clave:         imagen.Clave,
							Diagnostico: models.Diagnostico{
								Hallazgos:     diagnosticoReciente.Hallazgos,
								Impresion:     diagnosticoReciente.Impresion,
								Observaciones: diagnosticoReciente.Observaciones,
								Fecha:         diagnosticoReciente.Fecha,
								Medico:        diagnosticoReciente.Medico,
							},
						})
					}

					serial++ // Incrementar el número de serie
				}
			}
		}
	}

	// Crear archivos de metadatos por carpeta
	for carpeta, metadatos := range metadatosPorCarpeta {
		metadataFileName := fmt.Sprintf("metadata/%s_Metadata.json", carpeta)
		metadataContent, err := json.MarshalIndent(metadatos, "", "  ")
		if err != nil {
			log.Fatalf("Error serializando los metadatos: %v", err)
		}

		w, err := zipWriter.Create(metadataFileName)
		if err != nil {
			log.Fatalf("Error creando archivo de metadatos %s en el ZIP: %v", metadataFileName, err)
		}

		if _, err := w.Write(metadataContent); err != nil {
			log.Fatalf("Error escribiendo metadatos en el archivo %s: %v", metadataFileName, err)
		}
		log.Printf("Archivo de metadatos %s añadido correctamente al ZIP.\n", metadataFileName)
	}

	// Finalizar el archivo ZIP
	log.Println("Proceso de creación de ZIP completado correctamente.")
}

// Función para eliminar todos los archivos en una carpeta
func EliminarArchivosEnCarpeta(carpeta string) error {
	// Lee todos los archivos en la carpeta
	archivos, err := os.ReadDir(carpeta)
	if err != nil {
		return err
	}

	// Recorre todos los archivos y elimínalos
	for _, archivo := range archivos {
		rutaArchivo := filepath.Join(carpeta, archivo.Name())
		err := os.Remove(rutaArchivo)
		if err != nil {
			return err
		}
	}

	return nil
}
