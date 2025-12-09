// Simple Node.js script to bundle JavaScript with esbuild
const esbuild = require('esbuild');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

// Ensure output directory exists
const distDir = path.join(__dirname, 'static', 'dist');
if (!fs.existsSync(distDir)) {
    fs.mkdirSync(distDir, { recursive: true });
}

const bundlePath = path.join(distDir, 'app.bundle.js');

esbuild.build({
    entryPoints: [path.join(__dirname, 'src', 'app.js')],
    bundle: true,
    format: 'iife', // Immediately Invoked Function Expression for browser
    outfile: bundlePath,
    target: 'es2020',
    platform: 'browser',
    sourcemap: false,
    minify: false, // No minification as requested
    treeShaking: true, // Remove unused code
    // External dependencies that should not be bundled (loaded separately)
    external: [],
}).then(() => {
    // Generate hash of the bundle for cache busting
    const bundleContent = fs.readFileSync(bundlePath);
    const hash = crypto.createHash('sha256').update(bundleContent).digest('hex').substring(0, 8);

    // Write version file
    const versionPath = path.join(distDir, 'version.txt');
    fs.writeFileSync(versionPath, hash);

    console.log('✓ JavaScript bundled successfully');
    console.log(`  Output: ${bundlePath}`);
    console.log(`  Version: ${hash}`);
}).catch((err) => {
    console.error('✗ Build failed:', err);
    process.exit(1);
});
