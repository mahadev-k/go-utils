# Go-Utils

A collection of **production-grade Go utilities** designed to reduce boilerplate, improve readability, and boost developer productivity — from clean workflow orchestration to **LLM-first token shrinking** 🚀

---

## 🧰 What’s inside?

* 🧵 **Task Runner** – sequential & parallel workflows with clean error handling
* 🔄 **Stream Utils** – functional-style map / filter pipelines
* 🗄️ **SQL Transaction Helpers** – composable transactional execution
* ⚙️ **YAML Config Loader** – layered configs with overrides
* 🤖 **LLM-Utils (TRON)** – token-efficient JSON encoding for LLMs

---

## 🧵 Task Runner

Run multiple steps in sequence **without verbose error handling** and without risking missed checks or variable shadowing.

### Before (verbose & error-prone)

```go
func FooBar() error {
	req := struct{
		isFoo bool
		isBar bool
	}{}
    ctx := context.TODO()
    err := processFoo(ctx, &req)
    if err != nil {
        return err
    }
    err = processBar(ctx, &req)
    if err != nil {
        return err
    }
    return nil
}
```

### After (clean & expressive)

```go
func FooBar() error {
	req := struct{
		isFoo bool
		isBar bool
	}{}
    ctx := context.TODO()
    runner := NewSimpleTaskRunner(ctx, req)
    _, err := runner.
        Then(processFoo).
        Then(processBar).
        Result()
    return err
}
```

### ⚡ Parallel execution

```go
req := struct{
		isFoo bool
		isBar bool
	}{}
runner := NewSimpleTaskRunner(ctx, req)
_, err := runner.
    Parallel(processFooParallel).
    Parallel(processBarParallel).
    Result()
```

---

## 🔄 Stream Utils (Map / Filter Pipelines)

Functional-style transformations inspired by streams & lambdas — but **idiomatic Go**.

```go
res, err := NewTransformer[string, int64](floatingStrings).
    Transform(MapIt[string, float64](strconv.ParseFloat)).
    Transform(MapIt[float64, float64](func(v float64) (float64, error) { return v * 10, nil })).
    Transform(MapIt[float64, int64](func(v float64) (int64, error) { return int64(v), nil })).
    Transform(FilterIt[int64](func(v int64) (bool, error) { return v%2 == 0, nil })).
    Result()
```

✔ Automatic error propagation
✔ No intermediate slices
✔ Readable, testable pipelines

---

## 🗄️ SQL Transaction Helpers

Compose **multi-step database transactions** without scattering rollback logic everywhere.

```go
err := dbutils.NewSqlTxnExec[OrderRequest, OrderProcessingResponse](
    context.TODO(), db, nil, &OrderRequest{CustomerName: "CustomerA"},
).
    StatefulExec(InsertOrder).
    StatefulExec(UpdateInventory).
    StatefulExec(InsertShipment).
    Commit()
```

✔ Automatic rollback on error
✔ Higher-order functions for clarity
✔ Ideal for service-layer code

---

## ⚙️ YAML Config Loader

Load configuration files with **override precedence** (perfect for env-based configs).

```go
_, err := yaml_configs.LoadConfigWithSuffix("./configs", "local")
```

```go
host := yaml_configs.Get[string]("database.host")
```

✔ Deterministic override order
✔ Typed accessors
✔ Zero boilerplate

---

## 🤖 LLM-Utils (TRON)

Utilities to **dramatically reduce token usage** when sending structured JSON payloads to LLMs.

### 🚀 TRON (Token Relational Object Notation)

TRON converts nested JSON into compact, relational-style tables:

* Nested objects → separate tables
* Parent-child relations → `<field>_id`
* Automatic row de-duplication
* Deterministic ordering (great for prompt caching)

### Example

Input JSON:

```json
{
  "people": [
    {"name":"p8","addresses":[{"street":"3rd","city":"la","country":{"code":"US","name":"usa"}}]}
  ]
}
```

TRON output:

```
name,addresses_id
p8,[1]
---addresses
city,street,country_id
la,3rd,1
---addresses.country
code,name
US,usa
```

💡 LLMs can now easily answer:
**“Where does p8 live?” → LA, 3rd Street, USA**

✔ 60–80% token reduction
✔ Human-readable
✔ LLM-reasoning friendly

---

## 📦 Install

```bash
go get -u github.com/mahadev-k/go-utils
```

---

## ⭐ Philosophy

> Write less glue code.
> Make workflows obvious.
> Optimize for both **humans and LLMs**.

If this repo saved you time, give it a ⭐ and build cooler Go systems ✨
