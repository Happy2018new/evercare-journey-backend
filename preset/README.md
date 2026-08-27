# Hot Place Preset

This directory contains a curated nationwide set of 65 well-known attractions
prepared for `database/define.HotPlace`.

## Files

- `hot_places.json`: records matching `define.HotPlace`. `RecommandDetail`
  contains the expanded attraction description and stays within the
  `varchar(2048)` character limit.
- `place_catalog.json`: the referenced `PlaceIdentity` plus basic POI location
  data used to create local `PlaceInfo` rows.
- `images/*.jpg`: one image per hot place. Every file is a JPEG with an exact
  `1920x1080` landscape canvas.
- `image_sources.json`: Wikimedia Commons source pages, selected image URLs,
  license/artist metadata when available, and the output dimensions.
- `main.go`: idempotently migrates the MySQL tables, imports the preset places
  and recommendations, and writes the images to the bbolt `PLACE_IMAGE`
  bucket. Run `go run ./preset` from the repository root.
- `collector/main.go`: repeatable collector and offline validator. Run
  `go run ./preset/collector -enrich` to refresh descriptions without network
  access, or `go run ./preset/collector -validate` to verify the existing pack.
  Use `-out` only when generating or validating an alternate data-pack location.

## Import Mapping

The initializer maps every `hot_places.json` record to its corresponding
`place_catalog.json` and `image_sources.json` entry. It creates an active local
`PlaceInfo` record with the `preset` provider, saves each image under the
matching `PlaceImageItemID`, then creates or updates the associated `HotPlace`
record. Rerunning the initializer updates the same records and resources.

## Source Note

Images were selected from Wikimedia Commons and normalized locally. Five
source lookups were rate-limited during the final collection run; their image
files are present and their records point to the corresponding Commons search
page with `source metadata deferred` in the license field. Refresh those five
records in `image_sources.json` before publishing attribution externally.
