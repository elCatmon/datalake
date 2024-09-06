import pydicom
import os
from pydicom.pixel_data_handlers.util import apply_voi_lut
import numpy as np
from PIL import Image

# Función para convertir de DICOM a JPG
def dicom_to_jpeg(file_path):
        # Obtener el directorio contenedor del archivo DICOM
        output_dir = os.path.dirname(file_path)
        # Crear el nombre del archivo de salida
        output_file = os.path.join(output_dir, os.path.splitext(os.path.basename(file_path))[0] + ".jpg")
        # Cargar el archivo DICOM
        ds = pydicom.dcmread(file_path)

        # Aplicar VOI LUT si está presente en el archivo DICOM
        if 'PixelData' in ds:
            try:
                data = apply_voi_lut(ds.pixel_array, ds)
            except Exception as e:
                print(f"Error al aplicar VOI LUT: {e}")
                data = ds.pixel_array
            # Normalizar los valores de píxeles a 8 bits
            if data.dtype != np.uint8:
                data = data.astype(np.float32)
                data = (np.maximum(data, 0) / data.max()) * 255.0
                data = np.uint8(data)
            # Convertir a imagen PIL
            img = Image.fromarray(data)
            # Convertir a escala de grises si la imagen no es en escala de grises
            if img.mode != 'L':
                img = img.convert('L')
            # Redimensionar la imagen si es necesario
            img = img.resize((128, 128), Image.LANCZOS)
            # Guardar la imagen como JPEG
            img.save(output_file, "JPEG")
        else:
            print("El archivo DICOM no contiene datos de píxeles (PixelData).")