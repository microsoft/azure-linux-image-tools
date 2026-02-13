set -eux
echo "Kangaroo" | tee --append /log.txt
echo "Working dir: $(pwd)" | tee --append /log.txt

# Ensure files in the config's directory are accessible to scripts.
# Use $0 instead of ${BASH_SOURCE[0]} for POSIX compatibility (dash on Ubuntu).
SCRIPT_DIR=$(dirname "$0")
cp "$SCRIPT_DIR/../files/a.txt" /
