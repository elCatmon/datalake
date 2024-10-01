package services

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"webservice/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

// Sube información de la donación física anonimizada con conversión de archivos JPG
func SubirDonacionFisica(datos []interface{}, w http.ResponseWriter, bucket *gridfs.Bucket, r *http.Request, database *mongo.Database) {
	log.Println("Iniciando proceso de subida de donación física.")

	estudioID, _ := datos[0].(string)
	donador, _ := datos[1].(string)
	estudio, _ := datos[2].(string)
	hash, _ := datos[3].(string)
	region, _ := datos[4].(string)
	valida, _ := datos[5].(string)
	sexo, _ := datos[6].(string)
	edad, _ := datos[7].(string)
	anonymizedFiles, _ := datos[8].([]*multipart.FileHeader)
	originalFiles, _ := datos[9].([]*multipart.FileHeader)

	log.Printf("EstudioID: %s, Donador: %s, Estudio: %s, Sexo: %s, Edad: %s, Archivos Anonimizados: %v, Archivos Originales: %v", estudioID, donador, estudio, sexo, edad, anonymizedFiles, originalFiles)

	// Clave única del estudio
	clave := estudio + region + "00" + valida + "0" + "1" + "0" + "0"
	log.Printf("Clave generada: %s", clave)

	if len(originalFiles) == 0 || len(anonymizedFiles) == 0 {
		http.Error(w, "Debe haber archivos originales y anonimizados", http.StatusBadRequest)
		log.Println("Error: No hay archivos originales o anonimizados.")
		return
	}

	var imagenes []models.Imagen

	// Subir archivos originales
	for _, fileHeader := range anonymizedFiles {

		log.Printf("Procesando archivo anonimizado: %s", fileHeader.Filename)

		// Crear una ruta temporal para el archivo
		tempFilePath := fmt.Sprintf("./archivos/%s", fileHeader.Filename)
		NameWithoutExt := filepath.Base(tempFilePath[:len(tempFilePath)-len(filepath.Ext(tempFilePath))])
		dcmFilePath := fmt.Sprintf("./archivos/%s.dcm", NameWithoutExt)

		// Guardar el archivo temporalmente
		if err := guardarArchivoTemporal(fileHeader, tempFilePath); err != nil {
			http.Error(w, "Error al guardar archivo temporal", http.StatusInternalServerError)
			return
		}

		// Convertir el archivo a JPG (si es necesario)
		if err := convertirArchivoJPG(tempFilePath, dcmFilePath); err != nil {
			http.Error(w, "Error al convertir archivo a JPG", http.StatusInternalServerError)
			return
		}

		// Subir archivo original a GridFS
		fileID, err := subirArchivoCGridFS(tempFilePath, bucket)
		if err != nil {
			http.Error(w, "Error al subir archivo anonimizado", http.StatusInternalServerError)
			return
		}

		// Subir archivo DICOM a GridFS
		dcmID, err := subirArchivoCGridFS(dcmFilePath, bucket)
		if err != nil {
			http.Error(w, "Error al subir archivo DICOM", http.StatusInternalServerError)
			return
		}

		// Limpiar archivos temporales
		defer os.Remove(tempFilePath)
		defer os.Remove(dcmFilePath)

		// Guardar los detalles de la imagen
		imagenes = append(imagenes, models.Imagen{
			Clave:       clave,
			Dicom:       dcmID,
			Imagen:      fileID,
			Anonimizada: true,
		})
		log.Printf("Archivo anonimizado subido y almacenado en GridFS con ID: %s", fileID)
	}

	// Subir archivos original (sin conversión)
	for _, fileHeader := range originalFiles {
		// Crear una ruta temporal para el archivo
		OtempFilePath := fmt.Sprintf("./archivos/%s", fileHeader.Filename)
		ONameWithoutExt := filepath.Base(OtempFilePath[:len(OtempFilePath)-len(filepath.Ext(OtempFilePath))])
		OdcmFilePath := fmt.Sprintf("./archivos/%s.dcm", ONameWithoutExt)

		// Guardar el archivo temporalmente
		if err := guardarArchivoTemporal(fileHeader, OtempFilePath); err != nil {
			http.Error(w, "Error al guardar archivo temporal", http.StatusInternalServerError)
			return
		}

		// Convertir el archivo a JPG (si es necesario)
		if err := convertirArchivoJPG(OtempFilePath, OdcmFilePath); err != nil {
			http.Error(w, "Error al convertir archivo a JPG", http.StatusInternalServerError)
			return
		}

		// Subir archivo DICOM a GridFS
		OdcmID, err := subirArchivoCGridFS(OdcmFilePath, bucket)
		if err != nil {
			http.Error(w, "Error al subir archivo DICOM", http.StatusInternalServerError)
			return
		}

		log.Printf("Subiendo archivo original: %s", fileHeader.Filename)
		fileID, err := subirArchivoGridFS(fileHeader, bucket)
		if err != nil {
			http.Error(w, "Error al subir archivo original", http.StatusInternalServerError)
			return
		}

		// Limpiar archivos temporales
		defer os.Remove(OtempFilePath)
		defer os.Remove(OdcmFilePath)

		imagenes = append(imagenes, models.Imagen{
			Clave:       clave,
			Dicom:       OdcmID,
			Imagen:      fileID,
			Anonimizada: false,
		})
		log.Printf("Archivo original subido y almacenado en GridFS con ID: %s", fileID)
	}

	// Crear documento del estudio
	estudioDoc := models.EstudioDocument{
		ID:        primitive.NewObjectID(),
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
			},
		},
	}

	log.Println("Insertando documento de estudio en la base de datos.")
	// Insertar el documento en la base de datos
	collection := database.Collection("estudios")
	if _, err := collection.InsertOne(r.Context(), estudioDoc); err != nil {
		http.Error(w, "Error al insertar documento", http.StatusInternalServerError)
		log.Printf("Error al insertar documento en la base de datos: %v", err)
		return
	}

	log.Println("Documento insertado exitosamente en la base de datos.")
}

// Guardar archivo temporalmente
func guardarArchivoTemporal(fileHeader *multipart.FileHeader, filePath string) error {
	log.Printf("Guardando archivo temporalmente: %s", filePath)
	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("Error al abrir archivo temporal: %v", err)
		return err
	}
	defer file.Close()

	out, err := os.Create(filePath)
	if err != nil {
		log.Printf("Error al crear archivo temporal: %v", err)
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		log.Printf("Error al copiar archivo temporal: %v", err)
	}
	return err
}

// Función para subir cualquier archivo a GridFS
func subirArchivoCGridFS(filePath string, bucket *gridfs.Bucket) (string, error) {
	log.Printf("Subiendo archivo a GridFS: %s", filePath)
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error al abrir archivo: %v", err)
		return "", err
	}
	defer file.Close()

	// Crear un stream de subida a GridFS
	uploadStream, err := bucket.OpenUploadStream(filepath.Base(filePath))
	if err != nil {
		log.Printf("Error al abrir stream de subida a GridFS: %v", err)
		return "", err
	}
	defer uploadStream.Close()

	// Copiar el contenido del archivo al stream de subida
	_, err = io.Copy(uploadStream, file)
	if err != nil {
		log.Printf("Error al copiar archivo al stream: %v", err)
		return "", err
	}

	log.Printf("Archivo subido a GridFS con éxito, ID: %s", uploadStream.FileID.(primitive.ObjectID).Hex())
	return uploadStream.FileID.(primitive.ObjectID).Hex(), nil
}

// Función para convertir el archivo a JPG
func convertirArchivoJPG(tempFilePath, jpgFilePath string) error {
	log.Printf("Ejecutando script de conversión para %s", tempFilePath)
	var cmd *exec.Cmd

	// Detectar el sistema operativo
	if runtime.GOOS == "windows" {
		cmd = exec.Command("python", "./services/jpg_dcm.py", tempFilePath, jpgFilePath)
	} else {
		cmd = exec.Command("python3", "./services/jpg_dcm.py", tempFilePath, jpgFilePath)
	}

	// Ejecutar el comando
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error al ejecutar el script de conversión: %v, output: %s", err, string(output))
		return fmt.Errorf("error al ejecutar el script de conversión: %w", err)
	}

	log.Printf("Script de conversión ejecutado correctamente para %s", tempFilePath)
	return nil
}

// Función para subir archivos a GridFS
func subirArchivoGridFS(fileHeader *multipart.FileHeader, bucket *gridfs.Bucket) (string, error) {
	log.Printf("Subiendo archivo a GridFS: %s", fileHeader.Filename)
	file, err := fileHeader.Open()
	if err != nil {
		log.Printf("Error al abrir archivo: %v", err)
		return "", err
	}
	defer file.Close()

	uploadStream, err := bucket.OpenUploadStream(fileHeader.Filename)
	if err != nil {
		log.Printf("Error al abrir stream de subida a GridFS: %v", err)
		return "", err
	}
	defer uploadStream.Close()

	_, err = io.Copy(uploadStream, file)
	if err != nil {
		log.Printf("Error al copiar archivo al stream: %v", err)
		return "", err
	}

	log.Printf("Archivo subido a GridFS con éxito, ID: %s", uploadStream.FileID.(primitive.ObjectID).Hex())
	return uploadStream.FileID.(primitive.ObjectID).Hex(), nil
}
