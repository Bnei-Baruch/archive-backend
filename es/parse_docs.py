#!/usr/bin/python2

import sys
from docx import Document

with open(sys.argv[1], 'rb') as f:
    document = Document(f)
    first = True
    for p in document.paragraphs:
        if not first:
            sys.stdout.write('\n')
        sys.stdout.write(p.text.encode('utf-8'))
        sys.stdout.flush()
        if first:
            first = False
