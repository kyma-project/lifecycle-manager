#!/usr/bin/env bash

## needed for tar to avoid unable to locale error
export LANG=de_DE.UTF-8 && export LC_ALL=$LANG

## usage:
## ./generate_oci_archives.sh <helm-chart-path> <crd-file-path> <archives-dst-dir>

helmChartDir=$(readlink -f "$1")
crdFilePath=$(readlink -f "$2")
archivesDstDir=$(readlink -f "$3")

if [ ! -d "$helmChartDir" ]; then
  echo "Error: $helmChartDir is not a directory"
  exit 1
fi

if [ ! -d "$archivesDstDir" ]; then
  echo "Error: $archivesDstDir is not a directory"
  exit 1
fi

if [ ! -f "$crdFilePath" ]; then
  echo "Error: $crdFilePath is not a file"
fi
cmd=gtar
## install Gnu-Tar if needed
if command -v $cmd >/dev/null 2>&1; then
    echo "Gnu-Tar is already installed: $(gtar --version)"
else
  brew install gnu-tar
  echo "Successfully installed Gnu-Tar: $(gtar --version)"
fi

## copy sample CRD
crdsDir="$helmChartDir"/crds
mkdir -p "$crdsDir" &&
  cp "$crdFilePath" "$crdsDir"/sample_crd.yaml

## generate helm chart archive
cd "$helmChartDir" && gtar czvf "$archivesDstDir"/helm_chart_with_crds.tgz .
lastCommandReturnCode=$?
if [ "$lastCommandReturnCode" -ne 0 ]; then
  echo "Failed to generate helm chart archive: $lastCommandReturnCode"
  echo "Cleaning up created directories"
  rm -rf "$crdsDir"
  exit 1
else
  echo "Generated helm chart archive"
fi

## rename crd file
mv "$crdsDir"/sample_crd.yaml "$crdsDir"/crd.yaml
## generate CRD archive
cd "$crdsDir" && gtar czvf "$archivesDstDir"/crd.tgz .
lastCommandReturnCode=$?
if [ "$lastCommandReturnCode" -ne 0 ]; then
  echo "Failed to generate CRD archive: $lastCommandReturnCode"
  echo "Cleaning up created directories"
  rm -rf "$crdsDir"
  exit 1
else
  echo "Generated CRD archive"
fi

echo "Cleaning up created directories"
rm -rf "$crdsDir"
