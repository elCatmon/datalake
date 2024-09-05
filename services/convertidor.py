#importacion de librerias
import pydicom
import xml.etree.ElementTree as ET
import os
from pydicom.dataset import Dataset, FileDataset
import base64
from pydicom.pixel_data_handlers.util import apply_voi_lut
import numpy as np
from PIL import Image
import html
from xml.dom import minidom
import datetime
from pydicom.uid import ExplicitVRLittleEndian

class conver():

    # Función para convertir de DICOM a XML
    def dicom_to_xml(file_path):
        meta, _ = os.path.splitext(file_path)
        ds = pydicom.dcmread(file_path)
        root = ET.Element("DicomData")
        # Iterar sobre todos los elementos del dataset DICOM
        for elem in ds:
            #obtener nombres de las etiquetas dicom
            tag = elem.tag
            tag_group = f"({tag.group:04X},{tag.element:04X})"
            tag_name = pydicom.datadict.keyword_for_tag(tag) or "Unknown Tag"
            vr = elem.VR
            vm = elem.VM
            # Valor de longitud calculado a partir del valor
            value = elem.value
            if isinstance(value, bytes):
                value_length = len(value)
            else:
                value_length = len(str(value))  # Longitud del valor en caracteres
            try:
                value_escaped = html.escape(str(value))  # Escapar caracteres especiales en XML
            except Exception as e:
                value_escaped = ""
            # Crear subelementos en el XML
            element = ET.SubElement(root, "Element")
            ET.SubElement(element, "TagGroup").text = tag_group
            ET.SubElement(element, "TagName").text = tag_name
            ET.SubElement(element, "VR").text = vr
            ET.SubElement(element, "VM").text = str(vm)
            ET.SubElement(element, "Length").text = str(value_length)
            ET.SubElement(element, "Value").text = value_escaped + "\n"
        # Convertir a cadena XML y aplicar formato
        xml_str = ET.tostring(root, encoding='unicode')
        xml_pretty_str = minidom.parseString(xml_str).toprettyxml(indent="  ")
        # Guardar el archivo XML
        with open(meta + ".xml", "w", encoding='utf-8') as xml_file:
            xml_file.write(xml_pretty_str)
        return 

    #Funcion para convertir de XML a DICOM
    def xml_to_dicom(xml_file_path):
        try:
            tree = ET.parse(xml_file_path)
        except ET.ParseError as e:
            print(f"Error parsing XML file: {e}")
            return

        root = tree.getroot()

        ds = FileDataset(xml_file_path, {}, file_meta=Dataset(), preamble=b"\0" * 128)

        for element in root.findall('Element'):
            tag_group = element.find('TagGroup').text
            tag_name = element.find('TagName').text
            vr = element.find('VR').text
            vm = element.find('VM').text
            value = element.find('Value').text

            # Eliminar posibles caracteres de nueva línea y espacios adicionales
            value = value.strip()

            tag = conver.get_tag(tag_group)

            # Parsear el valor según el VR
            parsed_value = conver.parse_value(vr, value)

            try:
                ds.add_new(tag, vr, parsed_value)
            except Exception as e:
                print(f"Error adding tag {tag} with VR={vr}: {e}")

        required_tags = [
            (0x0028, 0x0010),  # Rows
            (0x0028, 0x0011),  # Columns
            (0x7FE0, 0x0010),  # Pixel Data
            (0x0028, 0x0100),  # Bits Allocated
            (0x0028, 0x0101),  # Bits Stored
            (0x0028, 0x0102),  # High Bit
            (0x0028, 0x0103)   # Pixel Representation
        ]

        for tag in required_tags:
            if tag not in ds:
                print(f"Warning: Missing required tag {tag} in the dataset.")

        ds.is_little_endian = True
        ds.is_implicit_VR = True

        dicom_file_path = os.path.splitext(xml_file_path)[0] + "_reconstructed.dcm"
        ds.save_as(dicom_file_path)
        print(f"DICOM file saved as {dicom_file_path}")

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
        file_meta = pydicom.Dataset()
        file_meta.MediaStorageSOPClassUID = pydicom.uid.SecondaryCaptureImageStorage
        file_meta.MediaStorageSOPInstanceUID = pydicom.uid.generate_uid()
        file_meta.ImplementationClassUID = pydicom.uid.PYDICOM_IMPLEMENTATION_UID

        # Definir los detalles del archivo DICOM
        ds = FileDataset(dicom_file_path, {}, file_meta=file_meta, preamble=b"\0" * 128)

        # Establecer fechas y horas actuales
        dt = datetime.datetime.now()
        ds.ContentDate = dt.strftime('%Y%m%d')
        ds.ContentTime = dt.strftime('%H%M%S.%f')

        # Configurar datos del paciente (puedes ajustar estos datos según sea necesario)
        ds.PatientName = ""
        ds.PatientID = ""

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


    #Funcion para obtener etiquetas DICOM
    def get_tag(tag_group):
        group, element = tag_group.strip('()').split(',')
        return int(group, 16), int(element, 16)

    #Funcion para convertir valores de las etiquetas
    def parse_value(vr, value):
        if vr in ['OB', 'OW', 'OF', 'OD', 'UN', 'LT']:
            try:
                # Asume que el valor en el XML está codificado en base64
                return base64.b64decode(value)
            except Exception as e:
                print(f"Error decoding value for VR={vr}: {e}")
                return b''
        elif vr in ['US', 'UL', 'SL', 'SS']:
            try:
                # Asume que el valor en el XML es una lista separada por comas
                return [int(v) for v in value.strip('[]').split(',')]
            except ValueError as e:
                print(f"Error converting value for VR={vr}: {e}")
                return []
        elif vr in ['FL', 'FD']:
            try:
                # Asume que el valor en el XML es una lista separada por comas
                return [float(v) for v in value.strip('[]').split(',')]
            except ValueError as e:
                print(f"Error converting value for VR={vr}: {e}")
                return []
        elif vr in ['DS', 'IS']:
            # Asume que el valor en el XML es una lista separada por comas
            return [v.strip() for v in value.strip('[]').split(',')]
        else:
            # Maneja valores de texto directamente
            return value.strip()
