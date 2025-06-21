if [ "${VERSION}" = "" ] ; then
  VERSION="$(curl -si  https://api.github.com/repos/superplug-app/oasync/releases/latest | grep tag_name | sed -E 's/.*"([^"]+)".*/\1/')"
fi

echo "Downloading oasync version: $VERSION"

sudo curl -o /usr/bin/oasync -fsLO "https://github.com/superplug-app/oasync/releases/download/$VERSION/oasync"
sudo chmod +x /usr/bin/oasync