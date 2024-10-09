import pydicom
import sys
from pydicom.pixel_data_handlers.util import apply_voi_lut
import numpy as np
from PIL import Image

# Función para convertir de DICOM a JPG
def dicom_to_jpeg(input_file_path, output_file_path):
        # Cargar el archivo DICOM
        ds = pydicom.dcmread(input_file_path)
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
            img = img.resize((150, 150), Image.LANCZOS)
            # Guardar la imagen como JPEG
            img.save(output_file_path, "JPEG")
        else:
            print("El archivo DICOM no contiene datos de píxeles (PixelData).")

if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(1)

    input_file_path = sys.argv[1]
    output_file_path = sys.argv[2]
    dicom_to_jpeg(input_file_path, output_file_path)