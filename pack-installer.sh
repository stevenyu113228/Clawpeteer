#!/bin/bash
set -euo pipefail

# ============================================================
# Pack Clawpeteer installer into a zip for deployment
# ============================================================

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$PROJECT_DIR/build"
PACK_DIR="$BUILD_DIR/clawpeteer-install"
OUTPUT="$BUILD_DIR/clawpeteer-install.zip"

echo "Packing Clawpeteer installer..."

# Clean
rm -rf "$PACK_DIR" "$OUTPUT"
mkdir -p "$PACK_DIR"

# Copy install script
cp "$PROJECT_DIR/install/install.sh" "$PACK_DIR/"
chmod +x "$PACK_DIR/install.sh"

# Copy CLI (without node_modules and devDependencies)
mkdir -p "$PACK_DIR/cli"
cp "$PROJECT_DIR/cli/package.json" "$PACK_DIR/cli/"
cp -r "$PROJECT_DIR/cli/bin" "$PACK_DIR/cli/bin"
cp -r "$PROJECT_DIR/cli/src" "$PACK_DIR/cli/src"

# Copy SKILL.md
mkdir -p "$PACK_DIR/skill"
cp "$PROJECT_DIR/skill/SKILL.md" "$PACK_DIR/skill/"

# Copy CA cert if present
if [[ -f "$PROJECT_DIR/agent/certs/ca.crt" ]]; then
  cp "$PROJECT_DIR/agent/certs/ca.crt" "$PACK_DIR/ca.crt"
  echo "  Included ca.crt"
fi

# Create zip
cd "$BUILD_DIR"
zip -rq "clawpeteer-install.zip" "clawpeteer-install/"

# Show result
SIZE=$(du -h "$OUTPUT" | cut -f1)
echo ""
echo "Done! $OUTPUT ($SIZE)"
echo ""
echo "Deploy:"
echo "  scp $OUTPUT user@host:~/"
echo "  ssh user@host 'unzip clawpeteer-install.zip && cd clawpeteer-install && ./install.sh --broker-url mqtts://... --username ... --password ...'"
