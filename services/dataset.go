package services

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"webservice/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

func ObtenerArchivoDesdeGridFSDirecto(bucket *gridfs.Bucket, fileID primitive.ObjectID, archivoChan chan<- []byte, errChan chan<- error) {
	log.Printf("Iniciando descarga del archivo con ID: %v", fileID)

	var buf bytes.Buffer
	stream, err := bucket.OpenDownloadStream(fileID)
	if err != nil {
		errChan <- fmt.Errorf("error abriendo stream de descarga para el archivo con ID %v: %v", fileID, err)
		close(archivoChan)
		close(errChan)
		return
	}
	defer func() {
		if cerr := stream.Close(); cerr != nil {
			log.Printf("Error cerrando el stream de archivo con ID %v: %v", fileID, cerr)
		}
	}()

	log.Printf("Stream abierto correctamente para el archivo con ID: %v", fileID)

	_, err = io.Copy(&buf, stream)
	if err != nil {
		errChan <- fmt.Errorf("error copiando el stream del archivo con ID %v: %v", fileID, err)
		close(archivoChan)
		close(errChan)
		return
	}

	log.Printf("Archivo con ID %v descargado correctamente desde GridFS", fileID)
	archivoChan <- buf.Bytes()
	errChan <- nil

	close(archivoChan)
	close(errChan)
}

// Convención de nombre basada en la clave y número serial
func GenerarNombreArchivo(clave string, serial int) string {
	return fmt.Sprintf("%s_%d.dcm", clave, serial) // Usando la clave completa
}

// Función para crear el archivo JSON con los metadatos y agregarlo al zip
// Función para crear el archivo JSON con los metadatos y agregarlo al zip
func CrearArchivoJSON(zipWriter *zip.Writer, metadatos []models.ImagenMetadata, carpeta string) error {
	// Serializar metadatos a JSON
	data, err := json.MarshalIndent(metadatos, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando metadatos a JSON: %v", err)
	}

	// Crear el archivo JSON en el ZIP
	jsonWriter, err := zipWriter.Create(fmt.Sprintf("metadatos/%s_metadata.json", carpeta))
	if err != nil {
		return fmt.Errorf("error creando archivo JSON en ZIP: %v", err)
	}
	if _, err := jsonWriter.Write(data); err != nil {
		return fmt.Errorf("error escribiendo archivo JSON en ZIP: %v", err)
	}

	log.Printf("Archivo de metadatos JSON creado correctamente en el ZIP: %s_metadata.json.", carpeta)
	return nil
}

// ObtenerDiagnosticoMasReciente selecciona el diagnóstico más reciente basado en la fecha.
func ObtenerDiagnosticoMasReciente(diagnosticos []models.Diagnostico) models.Diagnostico {
	if len(diagnosticos) == 0 {
		return models.Diagnostico{}
	}

	diagnosticoMasReciente := diagnosticos[0]
	for _, diag := range diagnosticos {
		if diag.Fecha.After(diagnosticoMasReciente.Fecha) {
			diagnosticoMasReciente = diag
		}
	}
	return diagnosticoMasReciente
}

// CrearDiagnosticoSinMedico convierte un Diagnostico en DiagnosticoMetadata sin el campo Medico.
func CrearDiagnosticoSinMedico(diagnostico models.Diagnostico) models.DiagnosticoMetadata {
	return models.DiagnosticoMetadata{
		Hallazgos:     diagnostico.Hallazgos,
		Impresion:     diagnostico.Impresion,
		Observaciones: diagnostico.Observaciones,
		Fecha:         diagnostico.Fecha,
	}
}
