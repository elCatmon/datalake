package services

//Codigo generado por Cesar Ortega

// Importacion de librerias
import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
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
func RegistrarUsuario(db *mongo.Database, user models.User) (err error) {
	// Uso de defer para recuperar de cualquier pánico
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recuperado de pánico al registrar usuario: %v", r)
			err = fmt.Errorf("error inesperado al registrar usuario")
		}
	}()

	// Log de inicio del registro
	log.Printf("Intentando registrar usuario con correo: %s", user.Correo)

	collection := db.Collection("usuarios")

	// Validación de datos del usuario (puedes agregar más validaciones)
	if user.Nombre == "" || user.Correo == "" || user.Contrasena == "" {
		log.Printf("Datos incompletos para registrar usuario: %+v", user)
		return fmt.Errorf("datos incompletos para registrar usuario")
	}

	// Manejo de error en la inserción
	_, err = collection.InsertOne(context.TODO(), bson.M{
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

	// Uso de defer para recuperar de cualquier pánico
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recuperado de pánico al verificar correo: %v", r)
		}
	}()

	// Log de inicio de verificación
	log.Printf("Verificando existencia de correo: %s", email)

	// Validación de entrada
	if email == "" {
		log.Printf("El correo proporcionado está vacío")
		return false, fmt.Errorf("el correo proporcionado está vacío")
	}

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

	// Uso de defer para recuperar de cualquier pánico
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recuperado de pánico al validar usuario: %v", r)
		}
	}()

	// Log de inicio de validación
	log.Printf("Validando credenciales para el correo: %s", correo)

	// Validación de entrada
	if correo == "" || contrasena == "" {
		log.Printf("Correo o contraseña vacíos")
		return false, "", fmt.Errorf("correo o contraseña vacíos")
	}

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

// Cambiar la contraseña del usuario
func ChangePassword(db *mongo.Database, email, currentPassword, newPassword string) error {
	// Uso de defer para recuperar de cualquier pánico
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recuperado de pánico al cambiar la contraseña: %v", r)
		}
	}()

	// Validación de entrada
	if email == "" || currentPassword == "" || newPassword == "" {
		log.Printf("Email, contraseña actual o nueva contraseña vacíos")
		return fmt.Errorf("email, contraseña actual o nueva contraseña vacíos")
	}

	// Verificar si el correo electrónico existe
	exists, err := ExisteCorreo(db, email)
	if err != nil {
		return err
	}
	if !exists {
		log.Printf("Correo no encontrado: %s", email)
		return errors.New("correo no encontrado")
	}

	// Validar las credenciales del usuario
	valid, _, err := ValidarUsuario(db, email, currentPassword)
	if err != nil {
		return err
	}
	if !valid {
		log.Printf("La contraseña actual es incorrecta para el correo: %s", email)
		return errors.New("la contraseña actual es incorrecta")
	}

	// Hashear la nueva contraseña
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error al hashear la nueva contraseña: %v", err)
		return errors.New("error al cambiar la contraseña")
	}

	// Actualizar la contraseña en la base de datos
	collection := db.Collection("usuarios")
	_, err = collection.UpdateOne(context.TODO(), bson.M{"correo": email}, bson.M{"$set": bson.M{"contrasena": hashedPassword}})
	if err != nil {
		log.Printf("Error al actualizar la contraseña para el usuario con correo %s: %v", email, err)
		return errors.New("error al cambiar la contraseña")
	}

	log.Printf("Contraseña cambiada exitosamente para el usuario con correo: %s", email)
	return nil
}

// generadores
// HashContraseña genera el hash de una contraseña utilizando bcrypt.
func HashContraseña(password string) (string, error) {
	// Validación de entrada
	if password == "" {
		log.Printf("La contraseña está vacía")
		return "", fmt.Errorf("la contraseña está vacía")
	}

	// Uso de defer para recuperar de cualquier pánico
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recuperado de pánico al hashear la contraseña: %v", r)
		}
	}()

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error al hashear la contraseña: %v", err)
		return "", fmt.Errorf("error al hashear la contraseña: %v", err)
	}
	return string(bytes), nil
}

// Función para generar un hash SHA-256
// Función para generar un hash SHA-256
func GenerateHash(donador, numOperacion string) string {
	// Uso de defer para recuperar de cualquier pánico
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recuperado de pánico al generar hash: %v", r)
		}
	}()

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

	var imagenes []models.Imagen
	var anonymizedFiles []string
	var jpgFiles []string
	estudioID := primitive.NewObjectID().Hex()
	donador := "DonadorEjemplo" // Valor ejemplo, reemplazar por el valor correcto
	estudio, _ := getValueOrError(r.MultipartForm.Value, "tipoEstudio")
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

// Función para generar el dataset con programacion concurrente
func GenerarDataset(estudios []models.EstudioDocument, bucket *gridfs.Bucket, zipWriter *zip.Writer, tipoArchivo string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex

	serial := 1
	const maxConcurrency = 20 // Número máximo de goroutines concurrentes
	semaphore := make(chan struct{}, maxConcurrency)

	log.Println("Iniciando proceso para renombrar archivos y escribir en ZIP...")

	// Mapa para almacenar los metadatos por carpeta
	metadatosPorCarpeta := make(map[string][]models.ImagenMetadata)

	// Agregar README.txt en la raíz del ZIP
	readme := `Este dataset contiene estudios de iamgenes medicas anonimizadas
Los archivos estan nombrados siguiendo una convension de nombres que se describen el tipo de estudio, region del cuerpo, proyeccion
validez de la imagen, el origen de la imagen, su obtencion, sexo y edad los cuales se encuentran descritos en el archivo "nameconvention.txt"

La informacion para interpletar los nombres de los archivos estan en la carpeta "Metadata".`
	w, err := zipWriter.Create("README.txt")
	if err != nil {
		log.Printf("Error creando README.txt en el ZIP: %v", err)
		return err
	}
	if _, err := w.Write([]byte(readme)); err != nil {
		log.Printf("Error escribiendo README.txt en el ZIP: %v", err)
		return err
	}
	log.Println("README.txt añadido correctamente al ZIP en la raíz.")

	// Agregar nameconvention.txt en la carpeta 'Metadata'
	nameconvention := `Cada archivo tiene el siguiente formato: <tipo de estudio><region><proyeccion><valida><origen><obtencion><sexo><edad>_<identificador_secuencial>.dcm/.jpg
Ejemplo: 01020511100_0001.dcm
Desglose del nombre:
- Caracter 1 y 2: Tipo de estudio
    01 - Radiografía
    02 - Tomografía Computarizada
    03 - Resonancia Magnética<
    04 - Ultrasonido
    05 - Mamografía
    06 - Angiografía
    07 - Medicina Nuclear
    08 - Radio Terapia
    09 - Fluoroscopia
- Caracter 3 y 4: Region
    00 - Desconocido
    01 - Cabeza
    02 - Cuello
    03 - Torax
    04 - Pelvis
    05 - Brazo
    06 - Manos
    07 - Piernas
    08 - Rodilla
    09 - Tobillo
    10 - Pie
- Caracter 5 y 6: Proyeccion
    00 - Desconocido
    01 - Postero Anterior
    02 - Antero Posterior
    03 - Obliqua
    04 - Lateral Izquierda
    05 - Lateral Derecha
    06 - Especial
- Caracter 7: Valida
    0 - Si
    1 - No
- Caracter 8: Origen
    0 - Natural (Imagenes tomadas a pacientes)
    1 - Sintetico (Imagenes generadas por IA)
- Caracter 9: obtencion
    0 - donacion de empresa
    1 - donacion fisica
    2 - donacion digital
- Caracter 10: Sexo 
    0 - Desconocido
    1 - Masculino
    2 - Femenino
- Caracter 11: Edad (Rango de edades)
    0 - Desconocido
    1 - Lactantes (menores de 1 año)
    2 - Prescolar (1 a 5 años)
    3 - Infante (6 a 12 años)
    4 - Adolescente (13 a 18 años)
    5 - Adulto joven (19 a 26 años)
    6 - Adulto (27 a 59 años)
    7 - Adulto mayor (60 años y mas)
- Caracter _(12 - 16): identificador secuencial
	`
	w, err = zipWriter.Create("metadatos/nameconvention.txt")
	if err != nil {
		log.Printf("Error creando metadatos/nameconvention.txt en el ZIP: %v", err)
		return err
	}
	if _, err := w.Write([]byte(nameconvention)); err != nil {
		log.Printf("Error escribiendo metadatos/nameconvention.txt en el ZIP: %v", err)
		return err
	}
	log.Println("nameconvention.txt añadido correctamente al ZIP en la carpeta metadatos.")

	// Procesar estudios y generar archivos de imagen DICOM
	for _, estudio := range estudios {
		for _, imagen := range estudio.Imagenes {
			if imagen.Anonimizada && tipoArchivo == "dcm" && imagen.Dicom != primitive.NilObjectID.Hex() {
				nuevoNombre := GenerarNombreArchivo(imagen.Clave, serial)
				carpeta := nuevoNombre[:4] // Obtener la carpeta del nuevo nombre
				pOID, err := primitive.ObjectIDFromHex(imagen.Dicom)
				if err != nil {
					log.Printf("Error al convertir Dicom ID %v: %v", imagen.Dicom, err)
					continue
				}

				wg.Add(1)
				semaphore <- struct{}{}
				go func(nuevoNombre, carpeta string, pOID primitive.ObjectID) {
					defer wg.Done()                // Decrementar el contador del WaitGroup al final
					defer func() { <-semaphore }() // Liberar un espacio en el semáforo

					archivoChan := make(chan []byte) // Canal para recibir el archivo
					errChan := make(chan error)      // Canal para recibir errores

					// Llamar a la función que obtiene el archivo desde GridFS de forma concurrente
					go ObtenerArchivoDesdeGridFSDirecto(bucket, pOID, archivoChan, errChan)

					select {
					case archivoDicom := <-archivoChan:
						// Escribir el archivo en el ZIP de manera segura usando el mutex
						mu.Lock()
						defer mu.Unlock()

						// Definir la ruta donde se escribirá el archivo en el ZIP
						rutaArchivo := fmt.Sprintf("imagenes/%s/%s.dcm", carpeta, nuevoNombre)
						w, err := zipWriter.Create(rutaArchivo) // Crear el archivo en el ZIP
						if err != nil {
							log.Printf("Error creando archivo %s en el ZIP: %v", rutaArchivo, err)
							return
						}
						// Escribir los bytes del archivo DICOM en el ZIP
						if _, err := w.Write(archivoDicom); err != nil {
							log.Printf("Error escribiendo archivo %s en el ZIP: %v", rutaArchivo, err)
							return
						}
						log.Printf("Archivo %s añadido correctamente al ZIP en la carpeta 'imagenes/%s'.", nuevoNombre, carpeta)

						// Obtener el diagnóstico más reciente
						diagnosticoMasReciente := ObtenerDiagnosticoMasReciente(estudio.Diagnostico)

						// Crear una versión del diagnóstico sin el campo "Medico"
						diagnosticoSinMedico := CrearDiagnosticoSinMedico(diagnosticoMasReciente)

						// Crear el objeto de metadatos para esta imagen, excluyendo el campo "Medico"
						metadato := models.ImagenMetadata{
							NombreArchivo: nuevoNombre,
							Clave:         imagen.Clave,
							Diagnostico:   diagnosticoSinMedico, // Usar el diagnóstico sin el campo "realizo"
						}

						// Agregar los metadatos al mapa de la carpeta correspondiente
						metadatosPorCarpeta[carpeta] = append(metadatosPorCarpeta[carpeta], metadato)

					case err := <-errChan:
						// Manejar errores al obtener el archivo desde GridFS
						log.Printf("Error obteniendo archivo DICOM con ID %v: %v", pOID, err)
						return
					}
				}(nuevoNombre, carpeta, pOID)

				serial++
			}
		}
	}

	wg.Wait()

	// Escribir los archivos de metadatos por carpeta
	mu.Lock()
	for carpeta, metadatos := range metadatosPorCarpeta {
		if err := CrearArchivoJSON(zipWriter, metadatos, carpeta); err != nil {
			log.Printf("Error creando archivo de metadatos para carpeta %s: %v", carpeta, err)
		}
	}
	mu.Unlock()

	log.Println("Proceso de creación de ZIP completado correctamente.")
	return nil
}
