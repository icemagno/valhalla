package valhalla.client;

import com.sun.jna.Native;

import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;

public class ValhallaNativeLoader {

    private static volatile IValhallaLrc instance;

    public static IValhallaLrc getInstance() {
        if (instance == null) {
            synchronized (ValhallaNativeLoader.class) {
                if (instance == null) {
                    instance = loadNativeLibrary();
                }
            }
        }
        return instance;
    }

    private static IValhallaLrc loadNativeLibrary() {
        String osName = System.getProperty("os.name").toLowerCase();
        String libName;
        String extension;

        if (osName.contains("win")) {
            libName = "valhalla_lrc.dll";
            extension = ".dll";
        } else if (osName.contains("linux")) {
            libName = "libvalhalla_lrc.so";
            extension = ".so";
        } else if (osName.contains("mac")) {
            libName = "libvalhalla_lrc.dylib";
            extension = ".dylib";
        } else {
            throw new UnsupportedOperationException("Unsupported Operating System for Valhalla LRC: " + osName);
        }

        // Try extracting from embedded resource
        String resourcePath = "/libs/" + libName;
        InputStream in = ValhallaNativeLoader.class.getResourceAsStream(resourcePath);
        if (in == null) {
            in = ValhallaNativeLoader.class.getResourceAsStream("libs/" + libName);
        }

        if (in != null) {
            try {
                File tempFile = File.createTempFile("valhalla_lrc_", extension);
                tempFile.deleteOnExit();

                try (InputStream input = in; OutputStream out = new FileOutputStream(tempFile)) {
                    byte[] buffer = new byte[8192];
                    int bytesRead;
                    while ((bytesRead = input.read(buffer)) != -1) {
                        out.write(buffer, 0, bytesRead);
                    }
                }

                return Native.load(tempFile.getAbsolutePath(), IValhallaLrc.class);
            } catch (Exception e) {
                System.err.println("ValhallaNativeLoader: Failed to load extracted native library from temp file: " + e.getMessage());
            }
        }

        // Fallback: load directly from java.library.path or working directory
        return Native.load("valhalla_lrc", IValhallaLrc.class);
    }
}
