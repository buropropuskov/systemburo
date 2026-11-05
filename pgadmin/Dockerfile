FROM dpage/pgadmin4:9.19@sha256:3591b3d02910dcd817dec6fcaa681c278ba6966945283dd0ea0042731c330c83 AS production
USER root
COPY security-requirements.txt /tmp/security-requirements.txt
RUN /venv/bin/python3 -m pip install --no-cache-dir --only-binary=:all: --no-deps --require-hashes -r /tmp/security-requirements.txt \
    && /venv/bin/python3 -m pip check \
    && /usr/local/bin/python3 -m pip uninstall --yes pip \
    && rm -rf /usr/local/lib/python3.15/ensurepip \
    && rm /tmp/security-requirements.txt
USER 5050
