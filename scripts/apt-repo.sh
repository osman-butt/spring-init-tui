#!/usr/bin/env bash
# Turns a directory of Debian packages into a signed apt repository.
#
#   APT_SIGNING_KEY="$(cat private-key.asc)" scripts/apt-repo.sh <dir>
#
# <dir> must hold the packages somewhere below <dir>/pool. The indexes, their
# signatures, the public key and a page with the install commands are written
# next to it. Needs apt-ftparchive (Debian package apt-utils) and gpg.
set -euo pipefail

site="${1:?usage: apt-repo.sh <dir>}"
url="${APT_REPO_URL:-https://osman-butt.github.io/spring-init-tui}"
suite=stable
component=main
architectures=(amd64 arm64)

cd "$site"
if ! find pool -name '*.deb' | grep -q .; then
  echo "apt-repo.sh: no packages below $site/pool" >&2
  exit 1
fi

# The key lives only in a keyring that is deleted again.
GNUPGHOME="$(mktemp -d)"
export GNUPGHOME
trap 'rm -rf "$GNUPGHOME"' EXIT
gpg --batch --quiet --import <<<"${APT_SIGNING_KEY:?the private signing key}"

rm -rf dists
for arch in "${architectures[@]}"; do
  dir="dists/$suite/$component/binary-$arch"
  mkdir -p "$dir"
  apt-ftparchive --arch "$arch" packages pool >"$dir/Packages"
  gzip -9 --keep "$dir/Packages"
done

apt-ftparchive \
  -o "APT::FTPArchive::Release::Origin=si" \
  -o "APT::FTPArchive::Release::Label=si" \
  -o "APT::FTPArchive::Release::Suite=$suite" \
  -o "APT::FTPArchive::Release::Codename=$suite" \
  -o "APT::FTPArchive::Release::Components=$component" \
  -o "APT::FTPArchive::Release::Architectures=${architectures[*]}" \
  release "dists/$suite" >Release
mv Release "dists/$suite/Release"

# apt reads InRelease; Release.gpg is for older clients.
gpg --batch --yes --clearsign --output "dists/$suite/InRelease" "dists/$suite/Release"
gpg --batch --yes --armor --detach-sign --output "dists/$suite/Release.gpg" "dists/$suite/Release"
gpg --batch --armor --export >key.asc

cat >index.html <<EOF
<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>si apt repository</title>
<h1>si apt repository</h1>
<p>Debian packages of <a href="https://github.com/osman-butt/spring-init-tui">si</a>,
a terminal UI for Spring Initializr.</p>
<pre>
curl -fsSL $url/key.asc | sudo tee /etc/apt/keyrings/si.asc &gt; /dev/null
echo "deb [signed-by=/etc/apt/keyrings/si.asc] $url $suite $component" | sudo tee /etc/apt/sources.list.d/si.list
sudo apt update &amp;&amp; sudo apt install si
</pre>
</html>
EOF
