[![Build Status](https://travis-ci.org/husio/envconf.svg?branch=master)](https://travis-ci.org/husio/envconf)
[![GoDoc](https://godoc.org/github.com/husio/envconf?status.png)](https://godoc.org/github.com/husio/envconf)

# Load environment configuration


`envconf` provides straightforward way of loading [confiuration from
environment](https://12factor.net/config).


## Usage

Using example program `example.go`:

```go
package main

import (
        "fmt"

        "github.com/husio/envconf"
)

type configuration struct {
        HTTP        string
        AdminEmails []string `envconf:",required"`
        Cache       bool     `envconf:"USE_CACHE"`
}

func main() {
        conf := configuration{
                HTTP:     "localhost:8080",
                Cache: false,
        }
        envconf.Parse(&conf)

        fmt.Printf("HTTP = %v\n", conf.HTTP)
        fmt.Printf("AdminEmails = %v\n", conf.AdminEmails)
        fmt.Printf("Cache = %v\n", conf.Cache)
}
```


```bash
$ go build -o example example.go
$ ./example -h
HTTP          string       "localhost:8080"
ADMIN_EMAILS  string list  (required)
USE_CACHE     bool

$ ./example
Cannot parse configuration
  ADMIN_EMAILS: required

$ export ADMIN_EMAILS=foo@example.com,bar@example.com
$ ./example
HTTP = localhost:8080
AdminEmails = [foo@example.com bar@example.com]
Cache = false

$ export ADMIN_EMAILS=foo@example.com
$ export CACHE=t
$ export HTTP=0.0.0.0:12345
$ ./example
HTTP = 0.0.0.0:12345
AdminEmails = [foo@example.com]
Cache = false
```


## Customization via struct tag

You can change default behaviour using `envconf` tag.

First argument allows to overwrite default name. If empty, default value is
used.

If second argument is `required`, configuration loading will fail unless field
has non zero value.


## Supported types


* `string` and `[]byte`
* `bool`
* `int`, `int8`, `int16`, `int32`, `int64`
* `float32`, `float64`
* `encoding.TextUnmarshaler`
* slice of `string`, `bool`, `int`, `int8`, `int16`, `int32`, `int64`, `bool`, `float32`, `float64`
