#!/usr/bin/env bash

declare -A LANG_ANALYZERS
LANG_ANALYZERS['he']=hebrew
LANG_ANALYZERS['ru']=russian
LANG_ANALYZERS['es']=spanish
LANG_ANALYZERS['it']=italian
LANG_ANALYZERS['de']=german
LANG_ANALYZERS['nl']=dutch
LANG_ANALYZERS['fr']=french
LANG_ANALYZERS['pt']=portuguese
LANG_ANALYZERS['tr']=turkish
LANG_ANALYZERS['pl']=standard
LANG_ANALYZERS['ar']=arabic
LANG_ANALYZERS['hu']=hungarian
LANG_ANALYZERS['fi']=finnish
LANG_ANALYZERS['lt']=lithuanian
LANG_ANALYZERS['ja']=cjk
LANG_ANALYZERS['bg']=bulgarian
LANG_ANALYZERS['ka']=standard
LANG_ANALYZERS['no']=norwegian
LANG_ANALYZERS['sv']=swedish
LANG_ANALYZERS['hr']=standard
LANG_ANALYZERS['zh']=cjk
LANG_ANALYZERS['fa']=persian
LANG_ANALYZERS['ro']=romanian
LANG_ANALYZERS['hi']=hindi
LANG_ANALYZERS['ua']=standard
LANG_ANALYZERS['mk']=standard
LANG_ANALYZERS['sl']=standard
LANG_ANALYZERS['lv']=latvian
LANG_ANALYZERS['sk']=standard
LANG_ANALYZERS['cs']=czech

for k in ${!LANG_ANALYZERS[@]}; do
    sed "s/english/${LANG_ANALYZERS[$k]}/g" classification/classification-en.json > classification/classification-${k}.json;
done
