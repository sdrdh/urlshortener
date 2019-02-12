# Simple URL Shortener

Simple URL shortener which uses [MongoDB](https://www.mongodb.com/) as the Persistance. Uses MD5 hash on the url and takes 8 characters from it. And when the request comes with the shortened URL it gets the corresponding URL and issues a TemporaryRedirect to the URL.

### How to run
```shell
$ go run main.go -p {PORT} -d {MONGOURI}
#or if you want to build binary
$ go build -o urlshortener .
$ ./urlshortener -p {PORT} -d {MONGOURI}
#and if you set the env variables PORT, DB_URI
$ ./urlshortener
```

### How to use it

```shell
$ curl -X POST http://localhost:8080/shorten -d '{"url":"https://www.google.com","meta":{"blah":"blah"}}'
{"id":"8ffdefbd","url":"https://www.google.com","hitCount":0,"meta":{"blah":"blah"},"createdOn":1549995556}
$ curl http://localhost:8080/8ffdefbd
<a href="https://www.google.com">Temporary Redirect</a>.
```

