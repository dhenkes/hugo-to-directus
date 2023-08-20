# hugo-to-directus

This script takes a Hugo folder containing Markdown files, validates them and posts them to a Directus collection.

The body currently looks like this:

```json
{
  "title": "Title",
  "status": "published"
  "date": 1692515918360,
  "url": "unique-url",
  "content": "Content"
}
```
