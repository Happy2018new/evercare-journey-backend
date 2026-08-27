## Evercare journey backend
- Frontend: https://github.com/winterlove1026/EvercareJourneyPro
- Backend: https://github.com/Happy2018new/evercare-journey-backend

## Hot-place preset

With MySQL running and the connection values in `environment/db.go` available,
run the following command from the repository root to migrate the application
tables, import the 65 hot places, and write their images to `res.db`:

```powershell
go run ./preset
```

The command is idempotent for the bundled preset: rerunning it updates the
same preset place and hot-place records and overwrites their image resources.
