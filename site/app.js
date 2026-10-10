const repository = "kmoz000/PaieMaroc";
const fallback = `https://github.com/${repository}/releases`;

function wantedAsset(platform, assets) {
  const patterns = {
    Windows: [/Windows.*SELF-SIGNED-Setup\.exe$/i, /Windows.*SIGNED-Setup\.exe$/i],
    macOS: [/macOS.*SELF-SIGNED\.dmg$/i, /macOS.*SIGNED-NOTARIZED\.dmg$/i],
    Linux: [/Linux.*\.deb$/i],
  };
  return patterns[platform]
    .map((pattern) => assets.find((asset) => pattern.test(asset.name)))
    .find(Boolean);
}

function detectedPlatform() {
  const value = `${navigator.userAgent} ${navigator.platform}`;
  if (/Mac/i.test(value)) return "macOS";
  if (/Win/i.test(value)) return "Windows";
  if (/Linux|X11/i.test(value)) return "Linux";
  return null;
}

async function loadRelease() {
  const status = document.querySelector("#release-status");
  try {
    const response = await fetch(
      `https://api.github.com/repos/${repository}/releases?per_page=10`,
      {
        headers: { Accept: "application/vnd.github+json" },
      },
    );
    if (!response.ok) throw new Error(`GitHub ${response.status}`);
    const releases = await response.json();
    const release = releases.find((item) => !item.draft);
    if (!release) throw new Error("Aucune version publiée");
    document.querySelectorAll("[data-platform]").forEach((link) => {
      const asset = wantedAsset(link.dataset.platform, release.assets || []);
      if (asset) link.href = asset.browser_download_url;
    });
    const platform = detectedPlatform();
    const smartAsset = platform && wantedAsset(platform, release.assets || []);
    const smart = document.querySelector("#smart-download");
    if (smartAsset) {
      smart.href = smartAsset.browser_download_url;
      smart.firstChild.textContent = `Télécharger pour ${platform} `;
    }
    const channel = release.prerelease ? "version provisoire auto-signée" : "version officielle";
    status.textContent = `${release.tag_name} · ${channel} · publiée le ${new Intl.DateTimeFormat("fr-MA", { dateStyle: "long" }).format(new Date(release.published_at))}`;
  } catch (_) {
    status.textContent =
      "Les premiers binaires seront disponibles à la prochaine version.";
    document
      .querySelectorAll("[data-platform], #smart-download")
      .forEach((link) => {
        link.href = fallback;
      });
  }
}

loadRelease();
