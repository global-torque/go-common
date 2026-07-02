COMPANY_NAME=global-torque
SERVICE_NAME=go-common

case $1 in

run)
  GIT_COMMIT=$(git rev-parse --short HEAD)
  BUILD_DATE=$(date "+%Y%m%d")
  build && ./app
  ;;

audit)
  echo "running gosec"
  gosec ./...
  ;;

*)
  BRANCH_NAME=`git rev-parse --abbrev-ref HEAD`
  GIT_COMMIT=`git rev-parse --short HEAD`
  echo $BRANCH_NAME, $GIT_COMMIT
  docker build -t cr.webdevelop.pro/$COMPANY_NAME/$SERVICE_NAME:$GIT_COMMIT \
    -t cr.webdevelop.pro/$COMPANY_NAME/$SERVICE_NAME:latest-dev \
    --platform=linux/amd64 .
  # snyk container test cr.webdevelop.pro/$COMPANY_NAME/$SERVICE_NAME:latest-dev
  if [ $? -ne 0 ]; then
    echo "===================="
    echo "snyk has found a vulnerabilities, please consider choosing alternative image from snyk"
    echo "===================="
  fi
  docker push cr.webdevelop.pro/$COMPANY_NAME/$SERVICE_NAME:$GIT_COMMIT
  docker push cr.webdevelop.pro/$COMPANY_NAME/$SERVICE_NAME:latest-dev
  ;;

esac

