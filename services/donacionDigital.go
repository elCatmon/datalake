package services

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
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

// Función para ejecutar el script de anonimización
func anonimizarArchivo(tempFilePath, anonFilePath string) error {
	var cmd *exec.Cmd

	// Detectar el sistema operativo
	if runtime.GOOS == "windows" {
		cmd = exec.Command("python", "./services/anonimizacion.py", tempFilePath, anonFilePath)
	} else {
		pythonExecutable := "/home/upp05/mi_entorno/bin/python"

		// Crear el comando para ejecutar el script
		cmd = exec.Command(pythonExecutable, "./services/anonimizacion.py", tempFilePath, anonFilePath)
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
		pythonExecutable := "/home/upp05/mi_entorno/bin/python"

		// Crear el comando para ejecutar el script
		cmd = exec.Command(pythonExecutable, "./services/dcm_jpg.py", tempFilePath, jpgFilePath)
	}

	// Ejecutar el comando
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error al ejecutar el script de anonimización: %w", err)
	}

	return nil
}
