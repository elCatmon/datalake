package services

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Función para subir archivos a GridFS
func subirArchivoDigitalGridFS(filePath string, bucket *gridfs.Bucket) string {
	// Abrir el archivo para subir a GridFS
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Crear el archivo en GridFS
	uploadStream, err := bucket.OpenUploadStream(filepath.Base(filePath))
	if err != nil {
		return ""
	}
	defer uploadStream.Close()

	// Copiar el contenido del archivo al flujo de subida
	_, err = io.Copy(uploadStream, file)
	if err != nil {
		return ""
	}

	// Obtener el ID del archivo subido
	fileID := uploadStream.FileID.(primitive.ObjectID)
	return fileID.Hex()
}

func SubirArchivoConMetadatos(filePath string, bucket *gridfs.Bucket, estudioID string, anonimizada bool) string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error al abrir el archivo: %v", err)
		return ""
	}
	defer file.Close()

	uploadOpts := options.GridFSUpload().SetMetadata(bson.M{
		"estudio_ID":  estudioID,
		"anonimizada": anonimizada,
	})

	fileID, err := bucket.UploadFromStream(filepath.Base(filePath), file, uploadOpts)
	if err != nil {
		log.Printf("Error al subir archivo a GridFS: %v", err)
		return ""
	}

	return fileID.Hex()
}

// Función para ejecutar el script de anonimización
func anonimizarArchivo(tempFilePath, anonFilePath string) error {
	var cmd *exec.Cmd

	// Detectar el sistema operativo
	if runtime.GOOS == "windows" {
		cmd = exec.Command("python", "./services/anonimizacion.py", tempFilePath, anonFilePath)
	} else {
		cmd = exec.Command("python3", "./services/anonimizacion.py", tempFilePath, anonFilePath)
	}

	// Ejecutar el comando
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error al ejecutar el script de anonimización: %w", err)
	}
	return nil
}

func convertirArchivo(tempFilePath, jpgFilePath string) error {
	var cmd *exec.Cmd
	// Detectar el sistema operativo
	if runtime.GOOS == "windows" {
		cmd = exec.Command("python", "./services/dcm_jpg.py", tempFilePath, jpgFilePath)
	} else {
		cmd = exec.Command("python3", "./services/dcm_jpg.py", tempFilePath, jpgFilePath)
	}

	// Ejecutar el comando
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error al ejecutar el script de anonimización: %w", err)
	}

	return nil
}
