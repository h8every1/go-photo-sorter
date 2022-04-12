# photo-sorter

Sort files into subdirectories according to EXIF information

Arguments:

- `in` - input dir
- `out` - output dir. Defaults to `<inputDir>/sorted`

All image files in `inputDir` (non-recursive) are scanned for EXIF info and are moved
into `<outputDir>/<YYYY>/<YYYY-MM-DD>[/<CameraName>]/file.jpg`

Scans both `.jpg` and `.heic` files

Non-image files are moved into `<outputDir>/misc/<fileExtension>/<YYYY>/<YYYY-MM-DD>/file.jpg`

Skips `.exe`, `.bat`,`.ini`, `.dropbox` and `.app` files.