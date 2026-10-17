#!/usr/bin/env bash
# Genera el PDF del documento técnico a partir del Markdown.
#
# El documento se escribe en Markdown y se convierte con pandoc a un HTML
# autocontenido (estilos e imagen embebidos como data URI); Chrome en modo
# headless lo imprime a PDF. Así la conversión no depende de LaTeX ni de una
# suite de ofimática: solo de pandoc y de un navegador Chromium.
#
#   ./build-pdf.sh
#
# Requiere: pandoc en el PATH, google-chrome (o chromium) y pdfinfo (poppler,
# solo para informar el número de páginas).
set -euo pipefail

cd "$(dirname "$0")"

MD=documento-tecnico.md
CSS=documento-tecnico.css
PDF=documento-tecnico.pdf
HTML=$(mktemp /tmp/kite-doc-XXXXXX.html)
trap 'rm -f "$HTML"' EXIT

BROWSER=google-chrome
command -v "$BROWSER" >/dev/null 2>&1 || BROWSER=chromium
command -v "$BROWSER" >/dev/null 2>&1 || BROWSER=chromium-browser
if ! command -v "$BROWSER" >/dev/null 2>&1; then
  echo "no se encontró google-chrome ni chromium" >&2
  exit 1
fi

echo "→ pandoc: Markdown a HTML autocontenido"
pandoc "$MD" -o "$HTML" --standalone --embed-resources \
  --css="$CSS" --highlight-style=tango --toc --toc-depth=1

echo "→ $BROWSER (headless): HTML a PDF"
"$BROWSER" --headless --disable-gpu --no-sandbox \
  --no-pdf-header-footer --print-to-pdf="$PDF" "$HTML" >/dev/null 2>&1

paginas=$(pdfinfo "$PDF" 2>/dev/null | awk '/^Pages/{print $2}')
echo "→ listo: $PDF${paginas:+ ($paginas páginas)}"
