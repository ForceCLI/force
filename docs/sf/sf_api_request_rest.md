## sf api request rest

Make an authenticated HTTP request using the Salesforce REST API.

```
sf api request rest [url] [flags]
```

### Options

```
      --api-version string      Override the api version used for api requests made by this command
  -b, --body string             File or content for the body of the HTTP request.
  -f, --file string             JSON file that contains values for the request header, body, method, and URL.
  -H, --header stringArray      HTTP header in "key:value" format.
  -h, --help                    help for rest
  -i, --include                 Include the HTTP response status and headers in the output.
  -X, --method string           HTTP method for the request. (default "GET")
  -S, --stream-to-file string   Stream responses to a file.
  -o, --target-org string       Username or alias of the target org.
```

### SEE ALSO

* [sf api request](sf_api_request.md)	 - Make an authenticated HTTP request to your org.

