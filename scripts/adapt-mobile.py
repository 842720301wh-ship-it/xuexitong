"""Reproducible package adaptations after vendoring SuperStar."""
from pathlib import Path
import tarfile

root = Path(__file__).resolve().parents[1]
mobile = root / 'android/app/src/main/python'
archive = root / '.cache/python-vendor/pyaes-1.6.1.tar.gz'
if archive.exists():
    with tarfile.open(archive) as package:
        for member in package.getmembers():
            parts = Path(member.name).parts
            if not member.isfile() or len(parts) != 3 or parts[:2] != ('pyaes-1.6.1', 'pyaes') or not parts[2].endswith('.py'):
                continue
            target = mobile / 'pyaes' / parts[2]
            target.parent.mkdir(parents=True, exist_ok=True)
            with package.extractfile(member) as source:
                target.write_bytes(source.read())
        for member in package.getmembers():
            if member.isfile() and Path(member.name).name in ('LICENSE.txt', 'LICENSE'):
                (mobile / 'pyaes' / 'LICENSE.txt').write_bytes(package.extractfile(member).read())
for filename in (mobile / 'api').glob('*.py'):
    source = filename.read_text(encoding='utf-8')
    source = source.replace('"lxml"', '"html.parser"').replace("'lxml'", "'html.parser'")
    filename.write_text(source, encoding='utf-8')
