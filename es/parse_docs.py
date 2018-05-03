#!/usr/bin/python2

import sys
from docx import Document

with open(sys.argv[1], 'rb') as f:
    document = Document(f)
    text = '\n'.join([p.text.encode('utf-8') for p in document.paragraphs])
    sys.stdout.write(text)
    sys.stdout.flush()
