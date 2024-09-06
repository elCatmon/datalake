import pydicom
import numpy as np
from pydicom.dataset import FileDataset
import datetime
from pydicom.uid import ExplicitVRLittleEndian, generate_uid
from io import BytesIO
import sys

def jpg_to_dicom(image):
    try:
        # Convertir la imagen a escala de grises y redimensionar
        img = image.convert('L')
        img = img.resize((4096, 4096))

        np_img = np.array(img)

        # Crear el archivo DICOM en memoria
        file_meta = pydicom.Dataset()
        file_meta.MediaStorageSOPClassUID = pydicom.uid.SecondaryCaptureImageStorage
        file_meta.MediaStorageSOPInstanceUID = generate_uid()
        file_meta.ImplementationClassUID = pydicom.uid.PYDICOM_IMPLEMENTATION_UID

        ds = FileDataset("", {}, file_meta=file_meta, preamble=b"\0" * 128)

        # Establecer fechas y horas actuales
        dt = datetime.datetime.now()
        ds.ContentDate = dt.strftime('%Y%m%d')
        ds.ContentTime = dt.strftime('%H%M%S.%f')[:-3]  # Asegura el formato correcto

        # Configurar datos del paciente (puedes ajustar estos datos según sea necesario)
        ds.PatientName = ""
        ds.PatientID = ""

        # Configurar los parámetros del DICOM
        ds.Modality = "OT"  # Other
        ds.StudyInstanceUID = generate_uid()
        ds.SeriesInstanceUID = generate_uid()
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

        # Guardar en un buffer en memoria
        dicom_buffer = BytesIO()
        ds.save_as(dicom_buffer)

        dicom_buffer.seek(0)  # Volver al inicio del buffer
        return dicom_buffer
    except Exception as e:
        print(f"Error converting JPG to DICOM: {e}")
        sys.exit(1)
