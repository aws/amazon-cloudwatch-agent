import { Link } from "@mui/material";

export function ReleaseVersionIcon(): JSX.Element {
  return (
    <Link href="https://github.com/aws/amazon-cloudwatch-agent/releases">
      <img alt="Release Badge" src="https://img.shields.io/github/v/release/aws/amazon-cloudwatch-agent.svg" />
    </Link>
  );
}