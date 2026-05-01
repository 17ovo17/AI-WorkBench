set -e
rm -rf /opt/ai-workbench/prometheus-data /opt/ai-workbench/prometheus-import
mkdir -p /opt/ai-workbench/prometheus-data /opt/ai-workbench/prometheus-import
for f in /opt/ai-workbench/logs/*/*.om; do
  [ -s "$f" ] || continue
  name=$(basename "$(dirname "$f")")
  out="/opt/ai-workbench/prometheus-import/$name"
  rm -rf "$out"
  mkdir -p "$out"
  echo "import $f"
  if promtool tsdb create-blocks-from openmetrics "$f" "$out" >/tmp/promtool-$name.log 2>&1; then
    find "$out" -mindepth 1 -maxdepth 1 -type d -exec cp -a {} /opt/ai-workbench/prometheus-data/ \;
  else
    cat /tmp/promtool-$name.log
  fi
done
find /opt/ai-workbench/prometheus-data -maxdepth 2 -type f | head -20
