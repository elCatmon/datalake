import pydicom
import numpy as np
from pydicom.dataset import FileDataset
import datetime
<<<<<<< Updated upstream
from pydicom.uid import ExplicitVRLittleEndian

# Funcion para convertir de DICOM aJPG
def jpg_to_dicom(file_path):
        # Generación de rutas de archivos
        dicom_file_path = os.path.splitext(file_path)[0] + ".dcm"

        # Abrir la imagen JPG usando PIL
        img = Image.open(file_path)
        img = img.convert('L')  # Convertir a escala de grises

        # Redimensionar la imagen a 4096x4096
        img = img.resize((2048, 2048))

        np_img = np.array(img)

        # Crear un nuevo Dataset DICOM
=======
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
>>>>>>> Stashed changes
        file_meta = pydicom.Dataset()
        file_meta.MediaStorageSOPClassUID = pydicom.uid.SecondaryCaptureImageStorage
        file_meta.MediaStorageSOPInstanceUID = generate_uid()
        file_meta.ImplementationClassUID = pydicom.uid.PYDICOM_IMPLEMENTATION_UID

<<<<<<< Updated upstream
        # Definir los detalles del archivo DICOM
        ds = FileDataset(dicom_file_path, {}, file_meta=file_meta, preamble=b"\0" * 128)
=======
        ds = FileDataset("", {}, file_meta=file_meta, preamble=b"\0" * 128)
>>>>>>> Stashed changes

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

<<<<<<< Updated upstream
        # Ajustar la Transfer Syntax a Explicit VR Little Endian (una de las más comunes)
        ds.is_little_endian = True
        ds.is_implicit_VR = False

        # Añadir otros atributos obligatorios según el estándar DICOM
        ds.PatientOrientation = ""  # Orientación del paciente (puede dejarse vacío)
        ds.PositionReferenceIndicator = ""  # Indicador de referencia de posición (puede dejarse vacío)
        ds.ImagePositionPatient = [0.0, 0.0, 0.0]  # Posición del paciente (puede ajustarse según sea necesario)
        ds.ImageOrientationPatient = [1.0, 0.0, 0.0, 0.0, 1.0, 0.0]  # Orientación del paciente (valores por defecto)

        # Guardar el archivo DICOM con la Transfer Syntax seleccionada
        ds.file_meta.TransferSyntaxUID = ExplicitVRLittleEndian
        ds.save_as(dicom_file_path, write_like_original=False)
=======
        # Guardar en un buffer en memoria
        dicom_buffer = BytesIO()
        ds.save_as(dicom_buffer)

        dicom_buffer.seek(0)  # Volver al inicio del buffer
        return dicom_buffer
    except Exception as e:
        print(f"Error converting JPG to DICOM: {e}")
        sys.exit(1)
>>>>>>> Stashed changes
