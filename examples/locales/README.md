# Adding an interface language

UltimateWX loads optional interface translations from <data-dir>/locales/<language-code>.json when the panel starts. The default data directory in the Linux installer is /var/lib/ultimatewx; create /var/lib/ultimatewx/locales and copy a locale JSON file there, then reload the panel.

Use a lowercase ISO-style code such as nl or pt-BR as the filename (the language code may contain hyphen-separated subtags). The file format is:

```json
{
  "label": "Nederlands",
  "translations": {
    "status": "Status",
    "config": "Configuratie"
  }
}
```

label is shown in the language selector. Add any interface keys you can translate; missing keys fall back to English. Use the existing keys in internal/app/index.html as the reference. A file can also override or extend one of the built-in languages. Locale files are limited to 1 MiB, labels to 64 characters, and individual translations to 4096 characters.

The Dutch file in this directory is a small working example, not a complete translation.
