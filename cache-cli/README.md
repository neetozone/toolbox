# cache-cli

The `cache` command stores and restores dependency caches for NeetoCI jobs.

## Network timeouts (S3 backend)

Every S3 request is bounded, so an S3 endpoint that stops answering fails the
cache command instead of blocking the job until its own timeout.

| Limit | Default | What it bounds |
| --- | --- | --- |
| Connect | 10 s | Opening the TCP connection. |
| TLS handshake | 10 s | The HTTPS handshake. |
| Response headers | 30 s | Waiting for S3 to answer a request once it is sent. |
| Stall | 60 s | A connection that moves no bytes in either direction. |
| Attempts | 3 | Tries per request, with a backoff of at most 5 s between them. |

The stall limit only fires when a transfer makes no progress at all, so large
archives over slow links keep working: there is no limit on the total time of
an upload or a download.

Set `NEETO_CI_CACHE_HTTP_TIMEOUT` to a whole number of seconds to use that value
for both the response-header and the stall limits, for example
`NEETO_CI_CACHE_HTTP_TIMEOUT=120`. Invalid values are logged and ignored.

With the defaults, one request to an endpoint that accepts connections but
never answers fails after about 1.5 minutes (3 attempts of 30 s plus backoff).
A command makes a few such requests, so it gives up within a few minutes.

The SFTP backend bounds connecting (10 s) and the SSH handshake (the
response-header limit above). It does not detect a connection that stalls
after the handshake.

## Interrupted uploads (S3 backend)

Large archives are uploaded in parts. When an upload fails, `cache` asks S3 to
discard the parts already uploaded. A process that is killed cannot do that, and
its parts stay in the bucket, invisible to `cache list`, and are billed until
they are removed. Add a lifecycle rule to the cache bucket that removes them:

```json
{
  "Rules": [
    {
      "ID": "abort-incomplete-multipart-uploads",
      "Status": "Enabled",
      "Filter": {},
      "AbortIncompleteMultipartUpload": { "DaysAfterInitiation": 1 }
    }
  ]
}
```

```bash
aws s3api put-bucket-lifecycle-configuration --bucket <cache-bucket> \
  --lifecycle-configuration file://lifecycle.json
```

`put-bucket-lifecycle-configuration` replaces the bucket's existing rules, so
merge this rule into them first (`aws s3api get-bucket-lifecycle-configuration`).
