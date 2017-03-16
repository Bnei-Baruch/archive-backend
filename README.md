# Backend for new archive site

## Overview

Backend for new archive site including, ETLs from BB Metadata DB to Elasticsearch.

## Commands
The archive-backend is meant to be executed as command line.
Type `archive-backend <command> -h` to see how to use each command.


```Shell
archive-backend server
```

Execute the backend api server for the new archive site.


```Shell
archive-backend version
```

Print the version of archive-backend


## Configuration

The default config file is `config.toml` in your current work directory.

See `config.sample.toml` for a sample config file.


## Release and Deployment

Once development is done, all tests are green, we want to go live.
All we have to do is simply execute `misc/release.sh`.

To add a pre-release tag, add the relevant environment variable. For example,

```Shell
PRE_RELEASE=rc.1 misc/release.sh
```


## MDB models

When MDB schema is changed we need to update the `mdb` package. Run this script:

```Shell
misc/update_mdb_models.sh
```



## Elasticsearch related stuff
http://mrzard.github.io/blog/2015/03/25/elasticsearch-enable-mlockall-in-centos-7/

