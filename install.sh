if [ "${APINTSYNC_VERSION}" = "" ] ; then
  APINTSYNC_VERSION="$(curl -si  https://api.github.com/repos/apintegrate/apintsync/releases/latest | grep tag_name | sed -E 's/.*"([^"]+)".*/\1/')"
fi

echo "Downloading apintsync version: $APINTSYNC_VERSION"

sudo curl -o /usr/bin/apintsync -fsLO "https://github.com/apintegrate/apintsync/releases/download/$APINTSYNC_VERSION/apintsync"
sudo chmod +x /usr/bin/apintsync