#!/usr/bin/python

import sys
from docx import Document

f = open(sys.argv[1], 'rb')

document = Document(f)

text = '\n'.join([p.text for p in document.paragraphs])
print text.encode('utf-8')
