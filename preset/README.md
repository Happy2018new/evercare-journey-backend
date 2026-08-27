# Hot Place Preset

This directory contains a curated nationwide set of 65 well-known attractions
prepared for `database/define.HotPlace`.

## Files

- `hot_places.json`: records matching `define.HotPlace`. `RecommandDetail`
  contains the expanded attraction description and stays within the
  `varchar(2048)` character limit.
- `place_catalog.json`: the referenced `PlaceIdentity` plus Amap POI metadata
  used to create local `PlaceInfo` rows, including the provider place ID.
  `category_code` preserves the Amap provider code, while `category_name`
  contains exactly three semicolon-delimited, locally curated user-facing
  labels instead of the repetitive Amap category hierarchy.
- `images/*.jpg`: one image per hot place. Every file is a JPEG with an exact
  `1920x1080` landscape canvas.
- `image_sources.json`: Wikimedia Commons source pages, selected image URLs,
  license/artist metadata when available, and the output dimensions.
- `main.go`: idempotently migrates the MySQL tables, imports the preset places
  and recommendations, and writes the images to the bbolt `PLACE_IMAGE`
  bucket. Its bundled preset assets and default `res.db` location are resolved
  independently of the current working directory. It never calls the Amap
  service: it only imports the embedded data already present in this folder.
- `collector/main.go`: repeatable collector and offline validator. Run
  `go run ./preset/collector -enrich` to refresh descriptions without network
  access, `go run ./preset/collector -validate` to verify the existing pack,
  `go run ./preset/collector -categories` to refresh curated category labels
  without network access,
  or `go run ./preset/collector -amap` to refresh Amap POI metadata. The Amap
  refresh requires a configured Amap Web Service key; set it with
  `AMAP_WEB_SERVICE_KEY` to avoid placing it in source code. Use `-out` only
  when generating or validating an alternate data-pack location.

`go run ./preset` does not run the collector and does not consume Amap API
quota. Only the explicit `go run ./preset/collector -amap` command makes Amap
POI requests.

## Import Mapping

The initializer maps every `hot_places.json` record to its corresponding
`place_catalog.json` and `image_sources.json` entry. It creates an active local
`PlaceInfo` record with the `amap` provider and its Amap provider place ID,
saves each image under the matching `PlaceImageItemID`, then creates or updates
the associated `HotPlace` record. Rerunning the initializer updates the same
records and resources.

## Source Note

Images were selected from Wikimedia Commons and normalized locally. Five
source lookups were rate-limited during the final collection run; their image
files are present and their records point to the corresponding Commons search
page with `source metadata deferred` in the license field. Refresh those five
records in `image_sources.json` before publishing attribution externally.
