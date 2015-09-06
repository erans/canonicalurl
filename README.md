# canonicalurl

A RESTful service providing a URL's declared canonical URL based on a number of selectors; written in Go and runs on AppEngine (because I'm lazy and its easy).

If you wish to run it without AppEngine you need to change the use of urlfetch and memcache from AppEngine (shouldn't take too long).

The API supports fetching the canonical URL based on a numbers of selectors:
* link[rel='canonical'] - Fetches the declared canonical URL element
* meta[property='og:url'] - Fetches the declared canonical URL as defined by the OpenGraph protocol

Response can be JSON (default), JSONP (if <code>callback</code> parameter is set on the URL or plain text (for CLI usage).

Check out https://canonicalurl.info for further information and documentation of the API.

Written by Eran Sandler ([@erans](https://twitter.com/erans)) http://eran.sandler.co.il

