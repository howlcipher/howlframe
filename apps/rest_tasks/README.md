# REST Tasks

HTTP task CRUD using real REST methods and path parameters:

- `GET/POST /tasks`
- `GET/PATCH/DELETE /tasks/{id}`

Dogfoods `req_method`, `req_path`, `store_keys`, JSON bodies, and density sugar (`cond`, multi-let) inside route lambdas.

```bash
go run ../../howlframe.go -compile-bc rest_tasks.howl
go run ../../howlframe.go -run-bc -allow-caps network,database rest_tasks.howl.bc.bin
```
