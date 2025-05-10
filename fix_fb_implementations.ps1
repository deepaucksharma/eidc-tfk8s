# Script to fix function block implementation issues

# This script patches the function block implementations to address common issues:
# 1. BaseFunctionBlock initialization
# 2. Tracer.StartSpan parameter issues 
# 3. Metrics method names
# 4. Config field access

# Function to make simple replacements in files
function Replace-In-File {
    param (
        [string]$filePath,
        [string]$oldText,
        [string]$newText
    )
    
    $content = Get-Content -Path $filePath -Raw
    if ($content -match [regex]::Escape($oldText)) {
        $content = $content -replace [regex]::Escape($oldText), $newText
        Set-Content -Path $filePath -Value $content
        Write-Host "Updated: $filePath - Replaced: $oldText" -ForegroundColor Green
    } else {
        Write-Host "Pattern not found in $filePath - $oldText" -ForegroundColor Yellow
    }
}

# 1. Fix FB-GW implementation
# ============================

# Fix BaseFunction initialization
Replace-In-File "pkg\fb\gw\gw.go" "BaseFunctionBlock: fb.NewBaseFunctionBlock(""fb-gw"")," @"
BaseFunctionBlock: fb.BaseFunctionBlock{},
"@

# Initialize name and ready correctly
Replace-In-File "pkg\fb\gw\gw.go" "func (g *GW) Initialize(ctx context.Context) error {" @"
func (g *GW) Initialize(ctx context.Context) error {
	// Set the name and ready state
	baseFB := fb.NewBaseFunctionBlock("fb-gw")
	g.BaseFunctionBlock = baseFB
"@

# Fix tracer.StartSpan calls
Replace-In-File "pkg\fb\gw\gw.go" "ctx, span := g.tracer.StartSpan(ctx, ""gw.ProcessBatch"", nil)" "ctx, span := g.tracer.StartSpan(ctx, ""gw.ProcessBatch"")"
Replace-In-File "pkg\fb\gw\gw.go" "ctx, span := g.tracer.StartSpan(ctx, ""gw.ValidateBatch"", nil)" "ctx, span := g.tracer.StartSpan(ctx, ""gw.ValidateBatch"")"
Replace-In-File "pkg\fb\gw\gw.go" "ctx, span := g.tracer.StartSpan(ctx, ""gw.SendToDLQ"", nil)" "ctx, span := g.tracer.StartSpan(ctx, ""gw.SendToDLQ"")"
Replace-In-File "pkg\fb\gw\gw.go" "ctx, span := g.tracer.StartSpan(ctx, ""gw.ExportBatch"", nil)" "ctx, span := g.tracer.StartSpan(ctx, ""gw.ExportBatch"")"

# Fix metrics method name
Replace-In-File "pkg\fb\gw\gw.go" "g.metrics.RecordBatchValidationError(validationResult.Error.Error())" "g.metrics.RecordError(validationResult.Error.Error())"

# Fix config generation access
Replace-In-File "pkg\fb\gw\gw.go" "g.configGeneration =" "g.SetConfigGeneration("
Replace-In-File "pkg\fb\gw\gw.go" "g.configGeneration = generation" "g.SetConfigGeneration(generation)"

# Fix ready state access
Replace-In-File "pkg\fb\gw\gw.go" "g.BaseFunctionBlock.ready = true" "g.SetReady(true)"

# 2. Fix FB-RX implementation
# ============================

# Fix BaseFunction initialization
Replace-In-File "pkg\fb\rx\rx.go" "BaseFunctionBlock: fb.NewBaseFunctionBlock(""fb-rx"")," @"
BaseFunctionBlock: fb.BaseFunctionBlock{},
"@

# Initialize name and ready correctly
Replace-In-File "pkg\fb\rx\rx.go" "func (r *RX) Initialize(ctx context.Context) error {" @"
func (r *RX) Initialize(ctx context.Context) error {
	// Set the name and ready state
	baseFB := fb.NewBaseFunctionBlock("fb-rx")
	r.BaseFunctionBlock = baseFB
"@

# Fix tracer.StartSpan calls
Replace-In-File "pkg\fb\rx\rx.go" "ctx, span := r.tracer.StartSpan(ctx, ""rx.ProcessBatch"", nil)" "ctx, span := r.tracer.StartSpan(ctx, ""rx.ProcessBatch"")"
Replace-In-File "pkg\fb\rx\rx.go" "ctx, span := r.tracer.StartSpan(ctx, ""rx.ForwardBatch"", nil)" "ctx, span := r.tracer.StartSpan(ctx, ""rx.ForwardBatch"")"
Replace-In-File "pkg\fb\rx\rx.go" "ctx, span := r.tracer.StartSpan(ctx, ""rx.SendToDLQ"", nil)" "ctx, span := r.tracer.StartSpan(ctx, ""rx.SendToDLQ"")"
Replace-In-File "pkg\fb\rx\rx.go" "ctx, span := r.tracer.StartSpan(ctx, ""rx.UpdateConfig"", nil)" "ctx, span := r.tracer.StartSpan(ctx, ""rx.UpdateConfig"")"

# Fix config generation access
Replace-In-File "pkg\fb\rx\rx.go" "r.configGeneration =" "r.SetConfigGeneration("
Replace-In-File "pkg\fb\rx\rx.go" "r.configGeneration = generation" "r.SetConfigGeneration(generation)"

# Fix ready state access
Replace-In-File "pkg\fb\rx\rx.go" "r.BaseFunctionBlock.ready = true" "r.SetReady(true)"
Replace-In-File "pkg\fb\rx\rx.go" "r.ready = true" "r.SetReady(true)"

# Fix test DLQ field access
Replace-In-File "pkg\fb\rx\rx_test.go" "DLQ:    ""fb-dlq:5000""," "DLQ:    ""fb-dlq:5000"","

# 3. Fix FB-CL implementation
# ============================

# Fix BaseFunction initialization
Replace-In-File "pkg\fb\cl\classifier.go" "BaseFunctionBlock: fb.NewBaseFunctionBlock(""fb-cl"")," @"
BaseFunctionBlock: fb.BaseFunctionBlock{},
"@

# Initialize name and ready correctly
Replace-In-File "pkg\fb\cl\classifier.go" "func (c *Classifier) Initialize(ctx context.Context) error {" @"
func (c *Classifier) Initialize(ctx context.Context) error {
	// Set the name and ready state
	baseFB := fb.NewBaseFunctionBlock("fb-cl")
	c.BaseFunctionBlock = baseFB
"@

# Fix tracer.StartSpan calls
Replace-In-File "pkg\fb\cl\classifier.go" "ctx, span := c.tracer.StartSpan(ctx, ""cl.ProcessBatch"", nil)" "ctx, span := c.tracer.StartSpan(ctx, ""cl.ProcessBatch"")"
Replace-In-File "pkg\fb\cl\classifier.go" "ctx, span := c.tracer.StartSpan(ctx, ""cl.ForwardBatch"", nil)" "ctx, span := c.tracer.StartSpan(ctx, ""cl.ForwardBatch"")"
Replace-In-File "pkg\fb\cl\classifier.go" "ctx, span := c.tracer.StartSpan(ctx, ""cl.SendToDLQ"", nil)" "ctx, span := c.tracer.StartSpan(ctx, ""cl.SendToDLQ"")"
Replace-In-File "pkg\fb\cl\classifier.go" "ctx, span := c.tracer.StartSpan(ctx, ""cl.UpdateConfig"", nil)" "ctx, span := c.tracer.StartSpan(ctx, ""cl.UpdateConfig"")"

# Fix config generation access
Replace-In-File "pkg\fb\cl\classifier.go" "c.configGeneration =" "c.SetConfigGeneration("
Replace-In-File "pkg\fb\cl\classifier.go" "c.configGeneration = generation" "c.SetConfigGeneration(generation)"

# Fix ready state access
Replace-In-File "pkg\fb\cl\classifier.go" "c.BaseFunctionBlock.ready = true" "c.SetReady(true)"

# Declare and use variables
Replace-In-File "pkg\fb\cl\classifier.go" "// Declare and not used: piiFields" "// PII fields to check"
Replace-In-File "pkg\fb\cl\classifier.go" "// Declare and not used: salt" "// Salt used for PII hashing"

Write-Host "====================================================="
Write-Host "Function block implementation fixes completed!" -ForegroundColor Green
Write-Host "====================================================="
