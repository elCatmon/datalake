package services

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

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

// Crear los archivos de metadata
func CrearArchivosMetadata() error {
	log.Println("Creando archivos README.txt y nameconvention.txt...")
	readme := `Este dataset contiene estudios de iamgenes medicas anonimizadas
Los archivos estan nombrados siguiendo una convension de nombres que se describen el tipo de estudio, region del cuerpo, proyeccion
validez de la imagen, el origen de la imagen, su obtencion, sexo y edad los cuales se encuentran descritos en el archivo "nameconvention.txt"

La informacion para interpletar los nombres de los archivos estan en la carpeta "Metadata".`

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

	// Asegúrate de que la carpeta `./dataset/` existe, si no, créala
	if err := os.MkdirAll("./dataset/", 0755); err != nil {
		return err
	}

	// Guardar los archivos en la carpeta `./dataset/`
	if err := os.WriteFile("./dataset/README.txt", []byte(readme), 0644); err != nil {
		return err
	}
	if err := os.WriteFile("./dataset/nameconvention.txt", []byte(nameconvention), 0644); err != nil {
		return err
	}

	log.Println("Archivos de metadata creados correctamente en ./dataset/")
	return nil
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
