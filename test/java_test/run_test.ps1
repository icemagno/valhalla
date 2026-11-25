javac -cp "dist\java\valhalla-lrc.jar" -d "test\java_test" "test\java_test\JavaFederateTest.java"
$crcProc = Start-Process -FilePath "dist\bin\valhalla-crc.exe" -ArgumentList "-port", ":4005" -PassThru
Start-Sleep -Milliseconds 800
try {
    java -cp "test/java_test;dist/java/valhalla-lrc.jar" test.JavaFederateTest 127.0.0.1:4005
} finally {
    if ($crcProc -and -not $crcProc.HasExited) {
        Stop-Process -Id $crcProc.Id -Force
    }
}
