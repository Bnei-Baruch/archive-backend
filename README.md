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

(See the next section below for the instructions on installing Elasticsearch for Windows)

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

## Elasticsearch installation for Windows

1. Download and install the Java Virtual Machine for Windows from
http://www.oracle.com/technetwork/java/javase/downloads/jre8-downloads-2133155.html

![alt text](https://image.prntscr.com/image/PzmaOTOMQX2Bds_Dv_cXSA.png)

2. Download and install the Elasticsearch 5.6.0 MSI from
https://artifacts.elastic.co/downloads/elasticsearch/elasticsearch-5.6.0.msi

3. Open CMD as administrator

    1. Go to Elasticsearch bin directory

        ```Shell
        cd C:\Program Files\Elastic\Elasticsearch\bin
        ```

    2. To install analysis-phonetic type

        ```Shell
        elasticsearch-plugin install analysis-phonetic
        ```

    3. To install the hebrew plugin type

        ```Shell
        elasticsearch-plugin install https://bintray.com/synhershko/elasticsearch-analysis-hebrew/download_file?file_path=elasticsearch-analysis-hebrew-5.6.0.zip
        ```

    4. Answer 'y' to the security question

        ```Shell
        Continue with installation? [y/N]y
        ```

    5. To install ICU plugin type

        ```Shell
        elasticsearch-plugin install analysis-icu
        ```

4. Download and install Python - **version 2.7.x**
https://www.python.org/downloads/


5. Install python-docx (to get text from docx):

    * in CMD go to python directory

    ```Shell
    cd C:\Python27
    ```

    * and type

    ```Shell
    python -m pip install python-docx
    ```

6. Download and install LibreOffice (not OpenOffice!)

    https://www.libreoffice.org/donate/dl/win-x86_64/5.4.5/en-US/LibreOffice_5.4.5_Win_x64.msi

    Update 'soffice-bin' value with soffice.exe full path in config.toml, [elasticsearch] section:
    "C://Program Files//LibreOffice 5//program//soffice.exe"

7. Copy to config.toml the required commented-out lines from config.sample.toml that are related to Windows.

## License

MIT