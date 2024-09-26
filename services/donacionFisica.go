package services

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"webservice/models"

	"github.com/disintegration/imaging"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

// Sube informacion de la donacion fisica anonimizada
func SubirDonacionFisica(datos []interface{}, w http.ResponseWriter, bucket *gridfs.Bucket, r *http.Request, database *mongo.Database) {
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

	var imagenes []models.Imagen

	// Subir archivos originales
	for _, fileHeader := range originalFiles {
		log.Printf("Procesando archivos originales: %s", fileHeader.Filename)
		fileID, err := subirArchivoGridFS(fileHeader, bucket)
		if err != nil {
			http.Error(w, "Fallo al subir a la base de datos los archivos originales", http.StatusInternalServerError)
			return
		}

		imagenes = append(imagenes, models.Imagen{
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
		imagenes = append(imagenes, models.Imagen{
			Clave:       clave,
			Dicom:       fileID,
			Imagen:      fileID,
			Anonimizada: true,
		})
	}

	// Crear el documento del estudio
	estudioDoc := models.EstudioDocument{
		EstudioID: estudioID,
		Donador:   donador,
		Hash:      hash, // Asignar el hash generado
		Status:    0,
		Imagenes:  imagenes,
		Diagnostico: []models.Diagnostico{
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
