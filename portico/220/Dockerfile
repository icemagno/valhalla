FROM ubuntu:18.04

ENV LANG=C.UTF-8
ENV JAVA_HOME=/usr/lib/jvm/java-11-openjdk-amd64/
ENV RTI_HOME=/opt/portico-2.2.0
ENV RTI_RID_FILE=/opt/portico-2.2.0/RTI.rid
ENV PATH="${RTI_HOME}/lib:${PATH}"

RUN apt-get update && \
apt-get install -y --no-install-recommends openjdk-11-jdk git gcc g++ && \
cd /home/ && \
git clone https://github.com/openlvc/portico

COPY ./local.properties /home/portico/codebase

WORKDIR /home/portico/codebase

# Corrige o bug de target duplicado do Ant 1.10 no hla13 e ieee1516e
RUN sed -i 's/<extension-point name="compile"/<extension-point name="compile.ep"/g' /home/portico/codebase/profiles/linux/hla13.xml /home/portico/codebase/profiles/linux/ieee1516e.xml && \
    sed -i 's/extensionOf="compile"/extensionOf="compile.ep"/g' /home/portico/codebase/profiles/linux/hla13.xml /home/portico/codebase/profiles/linux/ieee1516e.xml

RUN sed -i '/-proc:full/d' /home/portico/codebase/profiles/system.macros.xml

RUN ./ant sandbox

RUN mv /home/portico/dist/portico-2.2.0/ /opt/

WORKDIR /opt/portico-2.2.0
COPY ./RTI.rid /opt/portico-2.2.0/RTI.rid
RUN rm -rf /home/portico

ENTRYPOINT ["/bin/bash"]





