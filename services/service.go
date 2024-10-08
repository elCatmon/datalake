package services

//Codigo generado por Cesar Ortega

// Importacion de librerias
import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
func RegistrarUsuario(db *sql.DB, user models.User) error {
	query := `INSERT INTO users (nombre, correo, contrasena) VALUES ($1, $2, $3)`
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

// generadores
// HashPassword genera el hash de una contraseña utilizando bcrypt.
func HashContraseña(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

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
		return nil, err
	}
	return downloadStream, nil
}

// Crear filtro de búsqueda de estudios basados en la clave personalizada
func CrearFiltro(w http.ResponseWriter, r *http.Request) (bson.M, error) {
	tipoEstudio := r.URL.Query().Get("tipoEstudio")
	region := r.URL.Query().Get("region")
	proyeccion := r.URL.Query().Get("proyeccion")

	// Crear el filtro de búsqueda para estudios con los filtros obligatorios
	filter := bson.M{
		"imagenes": bson.M{
			"$elemMatch": bson.M{
				"anonimizada": true, // Filtro obligatorio: imagen anonimizada
			},
		},
		"status": 1, // Filtro obligatorio: status activo
	}

	// Filtro obligatorio por tipo de estudio (primeros 2 dígitos de la clave)
	if tipoEstudio != "" {
		filter["imagenes.clave"] = bson.M{
			"$regex": "^" + tipoEstudio, // Filtro por tipo de estudio
		}
	} else {
		// Retornar error si el tipo de estudio no se especifica, ya que es obligatorio
		return nil, fmt.Errorf("el campo tipoEstudio es obligatorio")
	}

	// Filtros opcionales
	// Filtro por región (3to y 4to dígito de la clave)
	if region != "" {
		filter["imagenes.clave"] = bson.M{
			"$regex": "^.{2}" + region, // Filtra por región si está presente
		}
	}

	// Filtro por proyección (8vo y 9no dígito de la clave)
	if proyeccion != "" {
		filter["imagenes.clave"] = bson.M{
			"$regex": "^.{4}" + proyeccion, // Filtra por proyección si está presente
		}
	}
	return filter, nil
}

// Busca estudios aplicando el filtro
func BuscarEstudios(w http.ResponseWriter, studiesCollection *mongo.Collection, filter bson.M) ([]primitive.ObjectID, *mongo.Cursor, error) {
	// Buscar los estudios que cumplen con el filtro
	cursor, err := studiesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar estudios en la base de datos: "+err.Error(), http.StatusInternalServerError)
		return nil, nil, err // Retornar error después de enviar la respuesta
	}
	defer cursor.Close(context.Background())

	// Recolectar IDs de imágenes que cumplen con los criterios
	var imageIDs []primitive.ObjectID
	for cursor.Next(context.Background()) {
		var study models.EstudioDocument
		if err := cursor.Decode(&study); err != nil {
			http.Error(w, "Error al decodificar estudio: "+err.Error(), http.StatusInternalServerError)
			return nil, nil, err // Retornar error después de enviar la respuesta
		}

		for _, img := range study.Imagenes {
			if img.Anonimizada {
				if img.Imagen == "" {
					continue // Ignorar este ID y continuar con el siguiente
				}

				if len(img.Imagen) != 24 {
					continue // Ignorar este ID y continuar con el siguiente
				}

				imageID, err := primitive.ObjectIDFromHex(img.Imagen)
				if err != nil {
					continue // Ignorar este ID y continuar con el siguiente
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
		return nil, err // Retornar error después de enviar la respuesta
	}
	defer cursor.Close(context.Background())

	var images []string
	for cursor.Next(context.Background()) {
		var fileInfo models.FileDocument
		if err := cursor.Decode(&fileInfo); err != nil {
			http.Error(w, "Error al decodificar archivo: "+err.Error(), http.StatusInternalServerError)
			return nil, err // Retornar error después de enviar la respuesta
		}
		// Obtener el nombre del archivo y construir la URL
		filename := fileInfo.Filename
		if filename != "" {
			imageURL := config.GetIP() + "/image/" + filename
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

	datos := []interface{}{estudioID, donador, estudio, hash, "00", "0", sexo, edad, anonymizedFiles, originalFiles}

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

	var imagenes []models.Imagen
	var anonymizedFiles []string
	var jpgFiles []string
	estudioID := primitive.NewObjectID().Hex()
	donador := "DonadorEjemplo" // Valor ejemplo, reemplazar por el valor correcto
	estudio, err := getValueOrError(r.MultipartForm.Value, "tipoEstudio")
	clave := estudio + "00" + "00" + "0" + "0" + "1" + "0" + "0"
	hash := GenerateHash(donador, estudio)

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
		imagenes = append(imagenes, models.Imagen{
			Clave:       clave,
			Dicom:       fileID,
			Imagen:      "",
			Anonimizada: false,
		})
	}

	// Subir archivos anonimizados y JPG a GridFS
	for i := range anonymizedFiles {
		fileID := subirArchivoDigitalGridFS(anonymizedFiles[i], bucket)
		jpgID := subirArchivoDigitalGridFS(jpgFiles[i], bucket)

		if fileID == "" || jpgID == "" {
			http.Error(w, "Fallo al subir a la base de datos los archivos anonimizados", http.StatusInternalServerError)
			continue
		}
		imagenes = append(imagenes, models.Imagen{
			Clave:       clave,
			Dicom:       fileID,
			Imagen:      jpgID,
			Anonimizada: true,
		})
	}

	// Crear el documento del estudio
	estudioDoc := models.EstudioDocument{
		EstudioID: estudioID,
		Donador:   donador,
		Hash:      hash,
		Status:    0,
		Imagenes:  imagenes,
		Diagnostico: []models.Diagnostico{
			{
				Hallazgos:     "",
				Impresion:     "",
				Observaciones: "",
				Fecha:         time.Now(),
				Medico:        "",
			},
		},
	}

	// Insertar el documento en MongoDB
	collection := database.Collection("estudios")
	_, err = collection.InsertOne(context.Background(), estudioDoc)
	if err != nil {
		http.Error(w, "Fallo al insertar el documento", http.StatusInternalServerError)
		return err
	}

	// Eliminar archivos temporales
	for _, path := range append(anonymizedFiles, jpgFiles...) {
		if err := os.Remove(path); err != nil {
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Todos los archivos han sido procesados exitosamente y el estudio ha sido registrado."))
	return nil
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

// BuscarDiagnosticoReciente busca el diagnóstico más reciente de un estudio dado su _id
func BuscarDiagnosticoReciente(ctx context.Context, db *mongo.Database, id primitive.ObjectID) (*models.Diagnostico, error) {
	// Definir la colección
	collection := db.Collection("estudios")

	// Buscar el documento por _id
	var estudio models.EstudioDocument
	filter := bson.M{"_id": id}
	err := collection.FindOne(ctx, filter).Decode(&estudio)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("no se encontró el documento con el ID proporcionado")
		}
		return nil, err
	}

	// Si no tiene diagnósticos, regresar un error
	if len(estudio.Diagnostico) == 0 {
		return nil, errors.New("el estudio no tiene diagnósticos")
	}

	// Encontrar el diagnóstico más reciente
	var diagnosticoReciente models.Diagnostico
	for _, diag := range estudio.Diagnostico {
		if diag.Fecha.After(diagnosticoReciente.Fecha) {
			diagnosticoReciente = diag
		}
	}

	return &diagnosticoReciente, nil
}

// BuscarEstudioYDiagnostico busca el estudio y su diagnóstico más reciente utilizando el nombre de una imagen.
func BuscarEstudioYDiagnostico(imagenNombre string, db *mongo.Database) (string, *models.Diagnostico, error) {
	ctx := context.Background()

	// Buscar el ID del estudio basado en el nombre de la imagen
	estudioID, err := BuscarEstudioIDImagen(imagenNombre, db)
	if err != nil {
		return "", nil, fmt.Errorf("error al buscar el estudio: %v", err)
	}

	// Buscar el diagnóstico más reciente usando el ID del estudio
	diagnostico, err := BuscarDiagnosticoReciente(ctx, db, estudioID)
	if err != nil {
		return "", nil, fmt.Errorf("error al buscar el diagnóstico más reciente: %v", err)
	}

	// Buscar la clave de la imagen dentro del estudio
	studyCollection := db.Collection("estudios")
	var estudio models.EstudioDocument
	studyFilter := bson.M{"_id": estudioID}

	err = studyCollection.FindOne(ctx, studyFilter).Decode(&estudio)
	if err != nil {
		return "", nil, fmt.Errorf("error al obtener el estudio: %v", err)
	}

	// Encontrar la clave de la imagen correspondiente
	var claveImagen string
	for _, imagen := range estudio.Imagenes {
		if imagen.Imagen == imagenNombre {
			claveImagen = imagen.Clave
			break
		}
	}

	if claveImagen == "" {
		return "", nil, fmt.Errorf("no se encontró la clave de la imagen para el nombre: %s", imagenNombre)
	}

	// Retornar la clave de la imagen y el diagnóstico más reciente
	return claveImagen, diagnostico, nil
}
