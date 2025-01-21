if [ "${APINTSYNC_VERSION}" = "" ] ; then
  APINTSYNC_VERSION="$(curl -si  https://api.github.com/repos/apintegrate/apintegrate/releases/latest | grep tag_name | sed -E 's/.*"([^"]+)".*/\1/')"
fi

echo "Downloading apintegrate version: $APINTSYNC_VERSION"

sudo curl -o /usr/bin/apintegrate -fsLO "https://github.com/apintegrate/apintegrate/releases/download/$APINTSYNC_VERSION/apintegrate"
sudo chmod +x /usr/bin/apintegrate