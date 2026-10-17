"""Configuración de pytest.

Asegura que la raíz del servicio esté en ``sys.path`` para que los
tests puedan importar los paquetes ``pb`` y ``notifications`` con
independencia del directorio desde el que se invoque pytest.
"""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))
