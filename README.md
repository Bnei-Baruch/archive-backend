# MDB to Elasticsearch

## Overview

ETL from BB Metadata DB to Elasticsearch.



## Commands
The MDB2ES is meant to be executed as command line.
Type `mdb2es <command> -h` to see how to use each command.


```Shell
mdb2es esplorer
```

Execute the backend api server for the MDB Elasticsearch Explorer tool.


```Shell
mdb2es config <path>
```

Generate default configuration in the given path. If path is omitted STDOUT is used instead.
**Note** that default value to config file is `config.toml` in project root directory.


```Shell
mdb2es version
```

Print the version of MDB2ES



## Release and Deployment

Once development is done, all tests are green, we want to go live.
All we have to do is simply execute `misc/release.sh`.

To add a pre-release tag, add the relevant environment variable. For example,

```Shell
PRE_RELEASE=rc.1 misc/release.sh
```


## Elasticsearch related stuff
http://mrzard.github.io/blog/2015/03/25/elasticsearch-enable-mlockall-in-centos-7/