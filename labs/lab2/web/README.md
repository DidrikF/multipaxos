## Web Server

In this task you will learn how to implement a very simple web server. This
will be useful since you in a later lab most likely will have to construct one
as a user interface to your fault tolerant application.

Using the skeleton code found in `web/server.go`, implement the web server
specification found below. All patterns listed assume `localhost:8080` as root.
Go's http package is documented [here](http://golang.org/pkg/net/http/).

There are tests for each of the cases listed below in `server_test.go`. The
command `go test -v` will run all tests. As described in Lab 1, use the `-run`
flag to only run a specific test. Note that you do not need to start the web
server manually for running the provided tests. But, you may also manually test
your implementation using a web browser or the `curl` tool. In one terminal
window, start the web server by running the command `go run server.go`. The
server should then be available at `localhost:8080`.  For example, run `curl -v
localhost:8080` in another terminal window to manually verify the first item
of the web server specification. An example output is shown below.

```
$ curl -v localhost:8080
* Rebuilt URL to: localhost:8080/
*   Trying ::1...
* Connected to localhost (::1) port 8080 (#0)
> GET / HTTP/1.1
> User-Agent: curl/7.40.0
> Host: localhost:8080
> Accept: */*
>
< HTTP/1.1 200 OK
< Date: Sun, 18 Jan 2015 17:33:27 GMT
< Content-Length: 12
< Content-Type: text/plain; charset=utf-8
<
* Connection #0 to host localhost left intact
Hello World!%
```

#### Web server specification:

1. The pattern `/` (root) should return status code `200` and the body `Hello
   World!\n`.

1. The web server should count the number of HTTP requests made to it (to any
   pattern) since it started. This counter should be accessible at the pattern
   `/counter`. A request to this pattern should return status code `200` and
   the current count (inclusive the current request) as the body, e.g.
   `counter: 42\n.`

3. A request to the pattern `/lab2` should return status code `301` to the
   client with body `<a
   href=\"http://www.github.com/uis-dat520-s18/labs/tree/master/lab2\">Moved
   Permanently</a>.\n\n`.

4. The pattern `/fizzbuzz` should implement the Fizz buzz game. It should
   return "fizz" if the value is divisible by 3. It should return "buzz" if the
   value is divisible by 5. It should return "fizzbuzz" if the value is both
   divisible by 3 **and** 5. It should return the number itself for any other
   case. The value should be passed as a URL query string parameter named
   `value`. For example, a request to `/fizzbuzz?value=30` should return `200`
   and body `fizzbuzz\n`. A request to `/fizzbuzz?value=31` should return `200`
   and body `31\n`. The server should return status code `200` and the body
   `not an integer\n` if the `value` parameter can't be parsed as an integer.
   If the `value` parameter is empty, the server should return status code
   `200` and the body `no value provided\n`.

5. All other patterns should return status code `404` and the body `404 page
   not found\n`.

