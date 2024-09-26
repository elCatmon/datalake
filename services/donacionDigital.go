package services

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

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
