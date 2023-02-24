import { Link } from "@mui/material";

export function CodeCoverageIcon(): JSX.Element {
  return (
    <Link href="https://app.codecov.io/gh/aws/amazon-cloudwatch-agent">
      <img alt="CodeCov Badge" src="https://codecov.io/gh/aws/amazon-cloudwatch-agent/branch/main/graph/badge.svg" />
    </Link>
  );
}