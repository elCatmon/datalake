else if tipoArchivo == "jpg" && imagen.Imagen != (primitive.NilObjectID).Hex() {
	nuevoNombre = dataset.GenerarNombreArchivo(imagen.Clave, serial) // Solo el nombre base
	pOID, _ := primitive.ObjectIDFromHex(imagen.Dicom)

	// Obtener el archivo JPG desde GridFS usando el _id almacenado en imagen.Imagen
	archivoJPG, err := dataset.ObtenerArchivoDesdeGridFS(bucket, pOID)
	log.Printf(imagen.Dicom)
	if err != nil {
		log.Printf("Error obteniendo archivo JPG con ID %v: %v", imagen.Imagen, err)
		continue
	}
	

	// Asegúrate de que el nombre no contenga .dcm
	nuevoNombre = strings.TrimSuffix(nuevoNombre, ".dcm") // Eliminar .dcm si está presente

	// Crear el archivo dentro del ZIP con el nuevo nombre en la carpeta 'imagenes'
	w, err := zipWriter.Create("imagenes/" + nuevoNombre + ".jpg") // Agregar la extensión .jpg
	if err != nil {
		log.Fatalf("Error creando archivo %s en el ZIP: %v", nuevoNombre, err)
	}
	if _, err := w.Write(archivoJPG); err != nil {
		log.Fatalf("Error escribiendo archivo %s en el ZIP: %v", nuevoNombre, err)
	}
	log.Printf("Archivo JPG %s añadido correctamente al ZIP en la carpeta 'imagenes'.\n", nuevoNombre)

	// Añadir metadatos al JSON
	if len(estudio.Diagnostico) > 0 {
		diagnosticoReciente := estudio.Diagnostico[len(estudio.Diagnostico)-1]
		metadatos = append(metadatos, models.ImagenMetadata{
			NombreArchivo: nuevoNombre + ".jpg", // Asegúrate de incluir la extensión aquí también
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





var ip string = "http://34.82.40.214:80"


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



		jpgID := subirArchivoDigitalGridFS(jpgFiles[i], bucket)
