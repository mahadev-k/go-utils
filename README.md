# Go-Utils

go-utils is a library that is aimed provide useful libraries in go
to reduce the developer efforts on building stuffs and increasing 
productivity.

Few functionalities are mentioned below.

## Task Runner

The basic problem that this is trying to solve is how you want to run
multiple tasks based on a request that you recieved.
It's similar to jobs where you might want to run
`processA` followed by `processB` and so on.
All these process can result in an error. Golang is very verbose in error handling. Sometimes we dont want to see that redundant code.
Also this reduces the readability.
Once you handle the error for `processA` and you forgot for `processB`
Golang wont throw a compile time error causing you to miss this case.
A small miss can cause havoc. Though we are supposed to follow a lot of process before shipping to prod ask yourselves do you follow always or not? 
To solve this I have developed an approach where you will be 
more focussed on writing what matters and how easy would it be to look at a fn and understand what it does. This will also help in overcoming variable shadowing. Instances where we have multiple errors being assinged an error shadowing can occur and this can be bad. Following this pattern and right coding can help in avoiding such weird scenarios.

Examples -

#### A code with redundant error handling and reduced readability.

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

#### A code with the task runner

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
```

As you can observe how better the code is readable and executable. This thought process and framework can improve the readability of the code.

## Go-routine Enthusiasts

```go
func FooBar() error {
	req := struct{
		isFoo bool
		isBar bool
	}{}
	ctx := context.TODO()
	runner := NewSimpleTaskRunner(ctx, req)
	_, err := runner.
		Parallel(processFooParallel).
		Parallel(processBarParallel).
		Result()
	return err
}
```


## Stream Utils

We all know the famous lambdas and arrow functions. Golang
inherently doesnt support the arrow syntax. It would be nice to have
that in golang. For now suppose we need to do some Map operation that
is were things get hard. Well you are in for a cool implementation
from me to solve that for you. After this below implementation I would
ask you to think a soln of your own how this would have been implemented.

```go
func TestMapRunner(t *testing.T) {
	// Create a map with some values
	floatingStrings := []string{"0.1", "0.2", "22", "22.1"}

	res, err := NewTransformer[string, float64](floatingStrings).
		Map(MapIt[string, float64](func(item string) (float64, error) { return strconv.ParseFloat(item, 64) })).
		Map(MapIt[float64, float64](func(item float64) (float64, error) { return item * 10, nil })).
		Result()
	if err != nil {
		t.Errorf("Testcase failed with error : %v", err)
		return
	}
	// Output: [0.1 0.2 22 22.1]
	t.Logf("Result: %v", res)
	assert.ElementsMatch(t, []any{float64(1), float64(2), float64(220), float64(221)}, res)

}
```

The above example is for a conversion of `string` to `float64`.
This will handle the errors for you if there are any. The only exception will be there can be `runtime` errors if there is any
`Type Cast Issues` So be careful with this. Try to write the testcases
which should avoid this issue.

## Filter and Mapper Deadly Combo

An addition to the functionality is made now, filteration also works.
Happy time folks!!

```go
func TestFilterIt(t *testing.T) {
	// Create a map with some values
	floatingStrings := []string{"0.1", "0.2", "22", "22.1"}

	res, err := NewTransformer[string, int64](floatingStrings).
		Map(MapIt[string, float64](func(item string) (float64, error) {return strconv.ParseFloat(item, 64)})).
		Map(MapIt[float64, float64](func(item float64) (float64, error) { return item * 10, nil })).
		Map(MapIt[float64, int64](func(item float64) (int64, error) { return int64(item), nil })).
		Map(FilterIt[int64](func(item int64) (bool, error) { return item%2 == 0, nil })).
		Result()
	if err != nil {
		t.Errorf("Testcase failed with error : %v", err)
		return
	}
	// Output: [2 220]
	t.Logf("Result: %v", res)
	assert.ElementsMatch(t, []any{int64(2), int64(220)}, res)	
}
```

## Import library to your project to build cool stuff.

`go get -u github.com/mahadev-k/go-utils@v1.0.1`

Add this to your go.mod.
Use it as done in the examples module.

```go
module examples

go 1.23.2

require github.com/stretchr/testify v1.9.0

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mahadev-k/go-utils v1.0.1 // indirect *go-utils*
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
```

#### Simple example

```go
func TestMapRunnerLib(t *testing.T) {
	// Create a map with some values
	floatingStrings := []string{"0.1", "0.2", "22", "22.1"}

	res, err := streams.NewTransformer[string, int64](floatingStrings).
		Map(streams.MapIt[string, float64](func(item string) (float64, error) { return strconv.ParseFloat(item, 64) })).
		Map(streams.MapIt[float64, float64](func(item float64) (float64, error) { return item * 10, nil })).
		Map(streams.MapIt[float64, int64](func(item float64) (int64, error) { return int64(item), nil })).
		Map(streams.FilterIt[int64](func(item int64) (bool, error) { return item%2 == 0, nil })).
		Result()
	if err != nil {
		t.Errorf("Testcase failed with error : %v", err)
		return
	}
	// Output: [2 220]
	t.Logf("Result: %v", res)
	assert.ElementsMatch(t, []any{int64(2), int64(220)}, res)
}
```
