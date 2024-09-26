import pydicom
import os
import sys
from pydicom.dataset import FileDataset
import numpy as np
from PIL import Image
import datetime
from pydicom.uid import ExplicitVRLittleEndian
import logging

# Configurar el logging para que registre en la consola y en un archivo
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s', handlers=[
    logging.FileHandler("conversion_jpg_a_dicom.log"),
    logging.StreamHandler(sys.stdout)
])

# Función para convertir de JPG a DICOM
def jpg_to_dicom(input_file_path, output_file_path):
    try:
        # Validar la extensión del archivo de entrada
        logging.info(f"Iniciando conversión de JPG a DICOM para el archivo: {input_file_path}")

        # Abrir la imagen JPG usando PIL
        img = Image.open(input_file_path)
        logging.info(f"Archivo JPG abierto exitosamente: {input_file_path}")

        img = img.convert('L')  # Convertir a escala de grises
        logging.info("Imagen convertida a escala de grises")

        # Redimensionar la imagen a 4096x4096
        img = img.resize((4096, 4096))
        logging.info("Imagen redimensionada a 4096x4096")

        np_img = np.array(img)

        # Crear un nuevo Dataset DICOM
        file_meta = pydicom.Dataset()
        file_meta.MediaStorageSOPClassUID = pydicom.uid.SecondaryCaptureImageStorage
        file_meta.MediaStorageSOPInstanceUID = pydicom.uid.generate_uid()
        file_meta.ImplementationClassUID = pydicom.uid.PYDICOM_IMPLEMENTATION_UID

        # Definir los detalles del archivo DICOM
        ds = FileDataset(output_file_path, {}, file_meta=file_meta, preamble=b"\0" * 128)

        # Establecer fechas y horas actuales
        dt = datetime.datetime.now()
        ds.ContentDate = dt.strftime('%Y%m%d')
        ds.ContentTime = dt.strftime('%H%M%S.%f')

        # Configurar datos del paciente (puedes ajustar estos datos según sea necesario)
        ds.PatientName = "Desconocido"
        ds.PatientID = "00000"

        # Configurar los parámetros del DICOM
        ds.Modality = "OT"  # Other
        ds.StudyInstanceUID = pydicom.uid.generate_uid()
        ds.SeriesInstanceUID = pydicom.uid.generate_uid()
        ds.SOPInstanceUID = file_meta.MediaStorageSOPInstanceUID
        ds.SOPClassUID = file_meta.MediaStorageSOPClassUID

        # Especificar dimensiones de la imagen
        ds.SamplesPerPixel = 1
        ds.PhotometricInterpretation = "MONOCHROME2"
        ds.Rows, ds.Columns = np_img.shape
        ds.BitsAllocated = 8
        ds.BitsStored = 8
        ds.HighBit = 7
        ds.PixelRepresentation = 0
        ds.PixelData = np_img.tobytes()

        # Ajustar la Transfer Syntax a Explicit VR Little Endian
        ds.is_little_endian = True
        ds.is_implicit_VR = False

        # Añadir otros atributos obligatorios según el estándar DICOM
        ds.PatientOrientation = ""  # Orientación del paciente
        ds.PositionReferenceIndicator = ""  # Indicador de referencia de posición
        ds.ImagePositionPatient = [0.0, 0.0, 0.0]
        ds.ImageOrientationPatient = [1.0, 0.0, 0.0, 0.0, 1.0, 0.0]

        # Guardar el archivo DICOM
        ds.file_meta.TransferSyntaxUID = ExplicitVRLittleEndian
        ds.save_as(output_file_path, write_like_original=False)

        logging.info(f"Conversión completada exitosamente. Archivo DICOM guardado en: {output_file_path}")
        return output_file_path

    except Exception as e:
        logging.error(f"Error durante la conversión de JPG a DICOM: {str(e)}")
        return None

if __name__ == "__main__":
    if len(sys.argv) != 3:
        logging.error("Uso incorrecto. Se requieren dos argumentos: (ruta del archivo JPG, ruta del archivo de salida DICOM).")
        sys.exit(1)

    input_file_path = sys.argv[1]
    output_file_path = sys.argv[2]
    logging.info(f"Script iniciado con el archivo: {input_file_path}")
    jpg_to_dicom(input_file_path, output_file_path)
