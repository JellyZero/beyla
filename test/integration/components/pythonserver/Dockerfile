# Dockerfile that will build a container that runs python flask with gunicorn on port 8080
FROM python:3.11.4
EXPOSE 8080
RUN pip install flask gunicorn
# Alternative: RUN pip install flask uwsgi

# Set the working directory to /build
WORKDIR /build

# Copy the source code into the image for building
COPY test/ test/

WORKDIR /build/test/integration/components/pythonserver

CMD ["gunicorn", "-w", "4", "-b", "0.0.0.0:8080", "main:app"]
# CMD uwsgi --http 0.0.0.0:8080 --master -p 4 -w main:app