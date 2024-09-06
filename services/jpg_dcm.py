import pydicom
import os
from pydicom.dataset import FileDataset
import numpy as np
from PIL import Image
import datetime
from pydicom.uid import ExplicitVRLittleEndian
from pymongo import MongoClient
import gridfs
import sys

def jpg_to_dicom(file_path):
    try:
        dicom_file_path = os.path.splitext(file_path)[0] + ".dcm"

        img = Image.open(file_path)
        img = img.convert('L')  # Convertir a escala de grises
        img = img.resize((4096, 4096))

        np_img = np.array(img)

        # Crear el archivo DICOM
        file_meta = pydicom.Dataset()
        file_meta.MediaStorageSOPClassUID = pydicom.uid.SecondaryCaptureImageStorage
        file_meta.MediaStorageSOPInstanceUID = pydicom.uid.generate_uid()
        file_meta.ImplementationClassUID = pydicom.uid.PYDICOM_IMPLEMENTATION_UID

        ds = FileDataset(dicom_file_path, {}, file_meta=file_meta, preamble=b"\0" * 128)

        dt = datetime.datetime.now()
        ds.ContentDate = dt.strftime('%Y%m%d')
        ds.ContentTime = dt.strftime('%H%M%S.%f')

        # Establecer campos DICOM obligatorios
        ds.PatientName = ""
        ds.PatientID = ""
        ds.Modality = "OT"  # Other
        ds.StudyInstanceUID = pydicom.uid.generate_uid()
        ds.SeriesInstanceUID = pydicom.uid.generate_uid()
        ds.SOPInstanceUID = file_meta.MediaStorageSOPInstanceUID
        ds.SOPClassUID = file_meta.MediaStorageSOPClassUID

        ds.SamplesPerPixel = 1
        ds.PhotometricInterpretation = "MONOCHROME2"
        ds.Rows, ds.Columns = np_img.shape
        ds.BitsAllocated = 8
        ds.BitsStored = 8
        ds.HighBit = 7
        ds.PixelRepresentation = 0
        ds.PixelData = np_img.tobytes()

        ds.save_as(dicom_file_path)

        return dicom_file_path
    except Exception as e:
        print(f"Error converting JPG to DICOM: {e}")
        sys.exit(1)

def save_to_gridfs(file_path):
    try:
        client = MongoClient("mongodb://localhost:27017")
        db = client["bdmdm"]
        fs = gridfs.GridFS(db, collection="imagenes")

        with open(file_path, "rb") as file:
            file_id = fs.put(file, filename=os.path.basename(file_path))
        
        return file_id
    except Exception as e:
        print(f"Error saving file to GridFS: {e}")
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python script.py <image.jpg>")
        sys.exit(1)

    jpg_file_path = sys.argv[1]
    dicom_file_path = jpg_to_dicom(jpg_file_path)
    file_id = save_to_gridfs(dicom_file_path)
    print(file_id)  # Imprimir el file_id del DICOM almacenado en GridFS
