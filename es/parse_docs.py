#!/usr/bin/python2

import sys
from docx import Document

with open(sys.argv[1], 'rb') as f:
    document = Document(f)
    for p in document.paragraphs:
        sys.stdout.write(p.text.encode('utf-8'))
        sys.stdout.flush()
    # text = '\n'.join([p.text for p in document.paragraphs])
    # print(text.encode('utf-8'))
