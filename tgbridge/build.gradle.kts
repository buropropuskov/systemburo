plugins {
    java
}

group = "dev.keeq0"
version = "1.0.3"

repositories {
    mavenCentral()
    maven("https://repo.papermc.io/repository/maven-public/")
}

dependencies {
    compileOnly("io.papermc.paper:paper-api:26.3.build.16-alpha")
    compileOnly("com.google.code.gson:gson:2.11.0")
}

java {
    toolchain { languageVersion = JavaLanguageVersion.of(25) }
}

tasks.withType<JavaCompile> {
    options.encoding = "UTF-8"
    options.release = 25
}

tasks.jar {
    archiveFileName = "TgBridge-${project.version}.jar"
}
