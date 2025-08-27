# ChaosLabs CLI Tool

A command-line tool for verifying cryptographic signatures, checking file integrity, and comparing ChaosLabs exports.

## Features

- **Export Verification**: Verify cryptographic signatures and Merkle tree proofs
- **File Integrity**: Check checksums of all files in an export
- **Export Comparison**: Compare two exports and generate detailed difference reports
- **Download & Resume**: Download exports with resumable chunk support
- **Multiple Formats**: Support for NDJSON, Parquet, and CSV exports

## Installation

### From Source
```bash
git clone https://github.com/your-org/chaoslabs.git
cd chaoslabs/cli
go build -o chaoslabs-cli
```

### Pre-built Binaries
Download from [Releases](https://github.com/your-org/chaoslabs/releases):

```bash
# Linux
curl -L https://github.com/your-org/chaoslabs/releases/latest/download/chaoslabs-cli-linux-amd64 -o chaoslabs-cli
chmod +x chaoslabs-cli

# macOS
curl -L https://github.com/your-org/chaoslabs/releases/latest/download/chaoslabs-cli-darwin-amd64 -o chaoslabs-cli
chmod +x chaoslabs-cli

# Windows
curl -L https://github.com/your-org/chaoslabs/releases/latest/download/chaoslabs-cli-windows-amd64.exe -o chaoslabs-cli.exe
```

## Usage

### Verify Export Signature

Verify the cryptographic signature and Merkle tree of an export:

```bash
chaoslabs-cli verify --manifest manifest.json --public-key public.pem
```

### Check File Integrity

Verify that all files have correct checksums:

```bash
chaoslabs-cli check-files --manifest manifest.json --data-dir ./export-data/
```

### Compare Two Exports

Generate a detailed comparison report:

```bash
# Text output
chaoslabs-cli diff --export1 export1/manifest.json --export2 export2/manifest.json

# JSON output
chaoslabs-cli diff --export1 export1.ndjson --export2 export2.ndjson --format json --output diff-report.json

# Ignore specific fields
chaoslabs-cli diff --export1 export1.ndjson --export2 export2.ndjson --ignore-fields created_at,updated_at
```

### Show Export Information

Display detailed information about an export:

```bash
# Text format
chaoslabs-cli info --manifest manifest.json

# JSON format
chaoslabs-cli info --manifest manifest.json --format json
```

### Download Export

Download all chunks of an export:

```bash
chaoslabs-cli download --base-url https://chaoslabs.example.com --job-id export_123456 --output-dir ./downloads/
```

## Command Reference

### Global Flags

- `--verbose, -v`: Enable verbose output
- `--output, -o FILE`: Write output to file instead of stdout
- `--format, -f FORMAT`: Output format (text, json)

### verify

Verify export cryptographic signatures.

**Flags:**
- `--manifest, -m FILE`: Path to manifest.json file (required)
- `--public-key, -k FILE`: Path to public key file

**Example:**
```bash
chaoslabs-cli verify -m manifest.json -k public.pem
```

### check-files

Check file integrity using checksums.

**Flags:**
- `--manifest, -m FILE`: Path to manifest.json file (required)
- `--data-dir, -d DIR`: Directory containing export files (default: current directory)

**Example:**
```bash
chaoslabs-cli check-files -m manifest.json -d ./export-data/
```

### diff

Compare two exports and show differences.

**Flags:**
- `--export1 FILE`: Path to first export manifest or data file (required)
- `--export2 FILE`: Path to second export manifest or data file (required)
- `--ignore-fields FIELDS`: Comma-separated list of fields to ignore
- `--threshold FLOAT`: Similarity threshold for reporting (0.0-1.0, default: 0.95)

**Example:**
```bash
chaoslabs-cli diff --export1 old.ndjson --export2 new.ndjson --threshold 0.9
```

### info

Display export information.

**Flags:**
- `--manifest, -m FILE`: Path to manifest.json file (required)

**Example:**
```bash
chaoslabs-cli info -m manifest.json --format json
```

### download

Download and verify an export.

**Flags:**
- `--base-url URL`: Base URL of the ChaosLabs API (required)
- `--job-id ID`: Export job ID (required)
- `--output-dir, -o DIR`: Output directory (default: current directory)
- `--verify`: Verify file integrity after download (default: true)

**Example:**
```bash
chaoslabs-cli download --base-url https://api.chaoslabs.com --job-id export_123456
```

## Output Formats

### Text Format (Default)

Human-readable output suitable for terminal viewing:

```
Export Comparison Report
========================

Summary:
  Export 1 records: 1000
  Export 2 records: 1005
  Identical records: 950
  Modified records: 45
  Only in first: 5
  Only in second: 10
  Similarity score: 94.52%
  Status: ✗ DIFFERENT (below threshold 95.00%)
```

### JSON Format

Machine-readable JSON output for integration with other tools:

```json
{
  "export1": "export1.ndjson",
  "export2": "export2.ndjson",
  "summary": {
    "total_records_1": 1000,
    "total_records_2": 1005,
    "identical_records": 950,
    "modified_records": 45,
    "only_in_first": 5,
    "only_in_second": 10,
    "similarity_score": 0.9452
  },
  "differences": [...]
}
```

## Exit Codes

- `0`: Success
- `1`: General error
- `2`: Verification failed
- `3`: File integrity check failed
- `4`: Significant differences found (below threshold)

## Examples

### Complete Verification Workflow

```bash
# 1. Download export
chaoslabs-cli download --base-url https://api.chaoslabs.com --job-id export_123456 --output-dir ./audit/

# 2. Verify signature
chaoslabs-cli verify --manifest ./audit/manifest.json --public-key chaoslabs-public.pem

# 3. Check file integrity
chaoslabs-cli check-files --manifest ./audit/manifest.json --data-dir ./audit/

# 4. Compare with previous export
chaoslabs-cli diff --export1 ./previous/manifest.json --export2 ./audit/manifest.json --format json --output comparison.json
```

### CI/CD Integration

```bash
#!/bin/bash
set -e

# Download latest export
chaoslabs-cli download --base-url "$CHAOSLABS_API_URL" --job-id "$EXPORT_JOB_ID" --output-dir ./current/

# Verify integrity
chaoslabs-cli verify --manifest ./current/manifest.json --public-key ./keys/chaoslabs-public.pem
chaoslabs-cli check-files --manifest ./current/manifest.json --data-dir ./current/

# Compare with baseline
if [ -f "./baseline/manifest.json" ]; then
    chaoslabs-cli diff --export1 ./baseline/manifest.json --export2 ./current/manifest.json --threshold 0.95
    if [ $? -eq 4 ]; then
        echo "WARNING: Significant differences detected"
        exit 1
    fi
fi

echo "Export verification completed successfully"
```

### Audit Script

```bash
#!/bin/bash
# Comprehensive audit script

EXPORTS_DIR="./exports"
REPORTS_DIR="./reports"
THRESHOLD=0.98

mkdir -p "$REPORTS_DIR"

for export in "$EXPORTS_DIR"/*.json; do
    echo "Auditing $export..."
    
    # Generate info report
    chaoslabs-cli info --manifest "$export" --format json > "$REPORTS_DIR/$(basename "$export" .json)-info.json"
    
    # Verify signature
    if ! chaoslabs-cli verify --manifest "$export" --public-key ./public.pem; then
        echo "FAILED: Signature verification failed for $export"
        exit 1
    fi
    
    # Check files
    if ! chaoslabs-cli check-files --manifest "$export" --data-dir "$(dirname "$export")"; then
        echo "FAILED: File integrity check failed for $export"
        exit 1
    fi
done

echo "All exports passed audit"
```

## Troubleshooting

### Common Issues

**Error: "signature verification failed"**
- Ensure you have the correct public key
- Check that the export hasn't been tampered with
- Verify the manifest.json file is intact

**Error: "checksum mismatch"**
- File may have been corrupted during download
- Try re-downloading the specific chunk
- Check available disk space

**Error: "file not found"**
- Ensure all chunk files are in the specified data directory
- Check file permissions
- Verify the manifest.json file paths

### Debug Mode

Use verbose flag for detailed output:

```bash
chaoslabs-cli verify --manifest manifest.json --verbose
```

### Logging

Set environment variable for debug logging:

```bash
export CHAOSLABS_CLI_DEBUG=1
chaoslabs-cli verify --manifest manifest.json
```

## Integration with CI/CD

### GitHub Actions

```yaml
name: Verify ChaosLabs Export
on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Download ChaosLabs CLI
        run: |
          curl -L https://github.com/your-org/chaoslabs/releases/latest/download/chaoslabs-cli-linux-amd64 -o chaoslabs-cli
          chmod +x chaoslabs-cli
          
      - name: Verify Export
        run: |
          ./chaoslabs-cli download --base-url ${{ secrets.CHAOSLABS_API_URL }} --job-id ${{ secrets.EXPORT_JOB_ID }}
          ./chaoslabs-cli verify --manifest manifest.json --public-key .github/chaoslabs-public.pem
          ./chaoslabs-cli check-files --manifest manifest.json
```

### Jenkins Pipeline

```groovy
pipeline {
    agent any
    stages {
        stage('Verify Export') {
            steps {
                script {
                    sh '''
                        curl -L https://github.com/your-org/chaoslabs/releases/latest/download/chaoslabs-cli-linux-amd64 -o chaoslabs-cli
                        chmod +x chaoslabs-cli
                        ./chaoslabs-cli verify --manifest exports/manifest.json --public-key keys/public.pem
                        ./chaoslabs-cli check-files --manifest exports/manifest.json --data-dir exports/
                    '''
                }
            }
        }
    }
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
