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

### Plugins
1. Hebrew plugin:
  https://github.com/synhershko/elasticsearch-analysis-hebrew
1. Instead of standard analyzer for exact match (הריון to be same as היריון):
  ```Shell
  sudo bin/elasticsearch-plugin install analysis-phonetic
  ```
  https://www.elastic.co/guide/en/elasticsearch/plugins/current/analysis-phonetic.html

  WIP - Does not works yet.
1. ICU plugin to transliterate Russian (and others) to enable phonetic on them:
  ```Shell
  sudo bin/elasticsearch-plugin install analysis-icu
  ```
1. Ukrainial analyzer (fails for standard - Not started)

### Build index
There are two more dependencies required to build index:
1) Open Office (soffice binary) - to convert all doc to docx.
2) python-docx pyton library - to get text from docx
  - pip install python-docx



## License

MIT